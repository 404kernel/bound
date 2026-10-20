package dashboard

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/404kernel/bound/pkg/governor"
	"github.com/404kernel/bound/pkg/ledger"
	"github.com/404kernel/bound/pkg/vault"
)

//go:embed index.html
var IndexHTML string

type Dashboard struct {
	v   *vault.Vault
	gov *governor.Governor
	led *ledger.Ledger
}

func NewDashboard(v *vault.Vault, gov *governor.Governor, led *ledger.Ledger) *Dashboard {
	return &Dashboard{
		v:   v,
		gov: gov,
		led: led,
	}
}

func (d *Dashboard) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/dashboard", d.handleIndex)
	mux.HandleFunc("/bound/api/state", d.handleState)
	mux.HandleFunc("/bound/api/trip", d.handleTrip)
	mux.HandleFunc("/bound/api/reset", d.handleReset)
	mux.HandleFunc("/bound/api/simulate", d.handleSimulate)
}

func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(IndexHTML))
}

func (d *Dashboard) handleState(w http.ResponseWriter, r *http.Request) {
	tripped, tripReason := d.gov.Status()
	recentAudits, _ := d.led.Recent(30)
	recentTx := d.v.RecentTransactions(30)
	agentBreakdown := d.v.AgentBreakdown()

	summary := d.v.Summary()
	summary["breaker_tripped"] = tripped
	summary["trip_reason"] = tripReason
	summary["agent_breakdown"] = agentBreakdown
	summary["recent_transactions"] = recentTx
	summary["recent_audits"] = recentAudits
	summary["now"] = time.Now().Format(time.RFC3339)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (d *Dashboard) handleTrip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tripped, _ := d.gov.Status()
	if tripped {
		d.gov.ResetBreaker()
	} else {
		d.gov.Trip("Manual kill-switch engaged from control dashboard")
	}

	tripped, reason := d.gov.Status()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"breaker_tripped": tripped,
		"trip_reason":     reason,
	})
}

func (d *Dashboard) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_ = d.v.Reset()
	d.gov.ResetBreaker()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Vault balance and circuit breaker reset successfully.",
	})
}

type SimulateRequest struct {
	AgentID string  `json:"agent_id"`
	Target  string  `json:"target"`
	Amount  float64 `json:"amount"`
	Action  string  `json:"action"`
}

func (d *Dashboard) handleSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.AgentID = "demo-agent"
		req.Target = "api.openai.com"
		req.Amount = 0.045
		req.Action = "POST /v1/chat/completions"
	}
	if req.AgentID == "" {
		req.AgentID = "agent-demo"
	}
	if req.Target == "" {
		req.Target = "api.openai.com"
	}
	if req.Amount <= 0 {
		req.Amount = 0.015
	}
	if req.Action == "" {
		req.Action = "POST /v1/chat/completions"
	}

	// Evaluate through governor
	decision := d.gov.Evaluate(req.AgentID, req.Target, fmt.Sprintf("sim_%d", time.Now().UnixNano()), req.Amount)
	allowed := decision.Allowed

	if allowed {
		_ = d.v.Record(vault.Transaction{
			AgentID:     req.AgentID,
			Target:      req.Target,
			Amount:      req.Amount,
			Description: req.Action,
		})
	}

	_ = d.led.Log(ledger.AuditEntry{
		AgentID:   req.AgentID,
		Target:    req.Target,
		Action:    req.Action,
		Amount:    req.Amount,
		Allowed:   allowed,
		Reason:    decision.Reason,
		Remaining: d.v.Remaining(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"allowed":          allowed,
		"reason":           decision.Reason,
		"remaining_budget": d.v.Remaining(),
	})
}
