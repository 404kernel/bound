package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/404kernel/bound/pkg/governor"
	"github.com/404kernel/bound/pkg/ledger"
	"github.com/404kernel/bound/pkg/vault"
)

func TestProxyInterceptionAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	v := vault.NewVault(vault.Policy{
		MaxTotal:      50.00,
		MaxHourly:     10.00,
		MaxDaily:      25.00,
		MaxPerRequest: 5.00,
	}, filepath.Join(tmpDir, "vault.json"))

	gov := governor.NewGovernor(governor.DefaultConfig(), v)
	led := ledger.NewLedger(filepath.Join(tmpDir, "ledger.jsonl"))

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Hello"}}]}`))
	}))
	defer backend.Close()

	srv, err := NewServer(":0", gov, v, led, backend.URL)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	body := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("X-Bound-Agent", "test-agent-alpha")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.handleProxy(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from upstream, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestProxyInterceptionBlockedWhenTripped(t *testing.T) {
	tmpDir := t.TempDir()
	v := vault.NewVault(vault.Policy{
		MaxTotal:      10.00,
		MaxHourly:     5.00,
		MaxDaily:      10.00,
		MaxPerRequest: 5.00,
	}, filepath.Join(tmpDir, "vault.json"))

	gov := governor.NewGovernor(governor.DefaultConfig(), v)
	led := ledger.NewLedger(filepath.Join(tmpDir, "ledger.jsonl"))

	gov.Trip("Kill switch engaged manually")

	srv, err := NewServer(":0", gov, v, led, "http://127.0.0.1:9999")
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Bound-Agent", "test-agent-alpha")
	rr := httptest.NewRecorder()

	srv.handleProxy(rr, req)

	if rr.Code != http.StatusPaymentRequired && rr.Code != http.StatusForbidden {
		t.Fatalf("expected breaker trip status code, got %d", rr.Code)
	}
}
