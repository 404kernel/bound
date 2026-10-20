package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/404kernel/bound/pkg/dashboard"
	"github.com/404kernel/bound/pkg/governor"
	"github.com/404kernel/bound/pkg/ledger"
	"github.com/404kernel/bound/pkg/notify"
	"github.com/404kernel/bound/pkg/vault"
)

type Server struct {
	addr     string
	gov      *governor.Governor
	v        *vault.Vault
	led      *ledger.Ledger
	notif    *notify.Notifier
	server   *http.Server
	upstream *url.URL
}

func NewServer(addr string, gov *governor.Governor, v *vault.Vault, led *ledger.Ledger, defaultUpstream string) (*Server, error) {
	return NewServerWithNotifier(addr, gov, v, led, nil, defaultUpstream)
}

func NewServerWithNotifier(addr string, gov *governor.Governor, v *vault.Vault, led *ledger.Ledger, notif *notify.Notifier, defaultUpstream string) (*Server, error) {
	if addr == "" {
		addr = ":8443"
	}

	var upURL *url.URL
	var err error
	if defaultUpstream != "" {
		upURL, err = url.Parse(defaultUpstream)
		if err != nil {
			return nil, err
		}
	}

	s := &Server{
		addr:     addr,
		gov:      gov,
		v:        v,
		led:      led,
		notif:    notif,
		upstream: upURL,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/bound/status", s.handleStatus)
	mux.HandleFunc("/bound/reset", s.handleReset)
	mux.HandleFunc("/bound/check", s.handleCheck)
	mux.HandleFunc("/bound/api/telegram-webhook", s.handleTelegramWebhook)

	// Mount embedded real-time web dashboard
	dash := dashboard.NewDashboard(v, gov, led)
	dash.RegisterRoutes(mux)

	mux.HandleFunc("/", s.handleProxy)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s, nil
}

func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

func (s *Server) Close() error {
	return s.server.Close()
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	summary := s.v.Summary()
	summary["breaker_tripped"] = s.gov.BreakerTripped
	summary["trip_reason"] = s.gov.TripReason

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	_ = s.v.Reset()
	s.gov.ResetBreaker()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "ok", "message": "Bound vault and circuit breaker reset successfully."}`))
}

type CheckRequest struct {
	AgentID string  `json:"agent_id"`
	Target  string  `json:"target"`
	Amount  float64 `json:"amount"`
	Action  string  `json:"action"`
	Commit  bool    `json:"commit"`
}

type CheckResponse struct {
	Allowed         bool    `json:"allowed"`
	RequiresConfirm bool    `json:"requires_confirm"`
	Reason          string  `json:"reason"`
	RemainingBudget float64 `json:"remaining_budget"`
	CurrentTotal    float64 `json:"current_total"`
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" {
		req.AgentID = "default-agent"
	}
	if req.Target == "" {
		req.Target = "general"
	}

	decision := s.gov.Evaluate(req.AgentID, req.Target, fmt.Sprintf("chk_%d", time.Now().UnixNano()), req.Amount)

	// Trigger Escrow Alert if threshold exceeded
	if decision.RequiresConfirm && s.notif != nil {
		go func() {
			_ = s.notif.SendAlert(notify.EscrowRequest{
				ID:        fmt.Sprintf("escrow_%d", time.Now().UnixNano()),
				AgentID:   req.AgentID,
				Target:    req.Target,
				Amount:    req.Amount,
				Reason:    decision.Reason,
				Timestamp: time.Now(),
			})
		}()
	}

	if decision.Allowed && req.Commit {
		_ = s.v.Record(vault.Transaction{
			AgentID:     req.AgentID,
			Target:      req.Target,
			Amount:      req.Amount,
			Description: req.Action,
		})
	}

	_ = s.led.Log(ledger.AuditEntry{
		AgentID:   req.AgentID,
		Target:    req.Target,
		Action:    req.Action,
		Amount:    req.Amount,
		Allowed:   decision.Allowed,
		Reason:    decision.Reason,
		Remaining: s.v.Remaining(),
	})

	w.Header().Set("Content-Type", "application/json")
	if !decision.Allowed {
		w.WriteHeader(http.StatusPaymentRequired)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	_ = json.NewEncoder(w).Encode(CheckResponse{
		Allowed:         decision.Allowed,
		RequiresConfirm: decision.RequiresConfirm,
		Reason:          decision.Reason,
		RemainingBudget: s.v.Remaining(),
		CurrentTotal:    s.v.TotalSpent,
	})
}

func (s *Server) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var update struct {
		CallbackQuery *struct {
			ID   string `json:"id"`
			Data string `json:"data"`
			From struct {
				Username string `json:"username"`
			} `json:"from"`
		} `json:"callback_query"`
	}

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if update.CallbackQuery != nil {
		action := update.CallbackQuery.Data
		if strings.HasPrefix(action, "bound_approve:") {
			agent := strings.TrimPrefix(action, "bound_approve:")
			_ = s.led.Log(ledger.AuditEntry{
				AgentID:   agent,
				Target:    "telegram",
				Action:    "human_approval",
				Allowed:   true,
				Reason:    fmt.Sprintf("Approved via Telegram by @%s", update.CallbackQuery.From.Username),
				Remaining: s.v.Remaining(),
			})
		} else if strings.HasPrefix(action, "bound_reject:") {
			agent := strings.TrimPrefix(action, "bound_reject:")
			s.gov.Trip(fmt.Sprintf("Halted via Telegram by @%s for agent %s", update.CallbackQuery.From.Username, agent))
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// 1. Determine Agent ID
	agentID := r.Header.Get("X-Bound-Agent-ID")
	if agentID == "" {
		agentID = "default-agent"
	}

	// 2. Determine target
	targetHost := r.Host
	if s.upstream != nil {
		targetHost = s.upstream.Host
	}

	// Read and buffer body for hashing & inspection
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	hash := sha256.Sum256(bodyBytes)
	hashStr := hex.EncodeToString(hash[:16])

	// Estimate cost based on path and payload length
	estimatedCost := 0.001 // minimum floor per request
	if strings.Contains(r.URL.Path, "chat/completions") || strings.Contains(r.URL.Path, "messages") {
		// Approximate ~4 chars per token
		tokens := len(bodyBytes) / 4
		if tokens < 100 {
			tokens = 100
		}
		estimatedCost = governor.EstimateTokenCost("gpt-4o", tokens, 200)
	}

	// 3. Evaluate with Bound Governor
	decision := s.gov.Evaluate(agentID, targetHost, hashStr, estimatedCost)
	if !decision.Allowed {
		s.led.Log(ledger.AuditEntry{
			AgentID:   agentID,
			Target:    targetHost,
			Action:    r.Method + " " + r.URL.Path,
			Amount:    decision.Cost,
			Allowed:   false,
			Reason:    decision.Reason,
			Remaining: s.v.Remaining(),
		})

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Bound-Status", "BLOCKED")
		w.WriteHeader(http.StatusPaymentRequired) // 402 Payment Required
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":            "bound_governor_rejected",
			"reason":           decision.Reason,
			"remaining_budget": s.v.Remaining(),
		})
		return
	}

	// 4. Reverse Proxy to Upstream
	targetURL := r.URL
	targetURL.Scheme = "https"
	if s.upstream != nil {
		targetURL.Scheme = s.upstream.Scheme
		targetURL.Host = s.upstream.Host
	} else if targetURL.Host == "" {
		targetURL.Host = targetHost
	}

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: targetURL.Scheme,
		Host:   targetURL.Host,
	})
	proxy.Transport = &http.Transport{
		Proxy: nil,
	}

	// Wrap response director to set headers
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		latency := time.Since(start).Milliseconds()
		resp.Header.Set("X-Bound-Status", "ALLOWED")
		resp.Header.Set("X-Bound-Remaining", fmt.Sprintf("%.4f", s.v.Remaining()))
		resp.Header.Set("X-Bound-Latency-Ms", fmt.Sprintf("%d", latency))

		// Commit spend to vault and ledger
		_ = s.v.Record(vault.Transaction{
			AgentID:     agentID,
			Target:      targetHost,
			Amount:      decision.Cost,
			Description: r.Method + " " + r.URL.Path,
		})

		_ = s.led.Log(ledger.AuditEntry{
			AgentID:   agentID,
			Target:    targetHost,
			Action:    r.Method + " " + r.URL.Path,
			Amount:    decision.Cost,
			Allowed:   true,
			Reason:    "Approved and routed",
			Remaining: s.v.Remaining(),
		})
		return nil
	}

	proxy.ServeHTTP(w, r)
}
