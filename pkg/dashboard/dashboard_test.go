package dashboard_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/404kernel/bound/pkg/dashboard"
	"github.com/404kernel/bound/pkg/governor"
	"github.com/404kernel/bound/pkg/ledger"
	"github.com/404kernel/bound/pkg/vault"
)

func setupTestDashboard(t *testing.T) (*dashboard.Dashboard, *http.ServeMux) {
	tmpDir := t.TempDir()
	vPath := filepath.Join(tmpDir, "vault.json")
	lPath := filepath.Join(tmpDir, "audit.jsonl")

	v := vault.NewVault(vault.Policy{
		MaxTotal:      50.00,
		MaxHourly:     10.00,
		MaxDaily:      20.00,
		MaxPerRequest: 10.00,
	}, vPath)

	gov := governor.NewGovernor(governor.DefaultConfig(), v)
	led := ledger.NewLedger(lPath)

	dash := dashboard.NewDashboard(v, gov, led)
	mux := http.NewServeMux()
	dash.RegisterRoutes(mux)

	return dash, mux
}

func TestDashboardHTMLServed(t *testing.T) {
	_, mux := setupTestDashboard(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "<title>Bound Dashboard</title>") && !strings.Contains(body, "BOUND // Control Dashboard") {
		t.Errorf("expected dashboard HTML title, got: %s", body[:100])
	}
}

func TestDashboardStateAPI(t *testing.T) {
	_, mux := setupTestDashboard(t)

	req := httptest.NewRequest(http.MethodGet, "/bound/api/state", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	if state["max_total"] != 50.0 {
		t.Errorf("expected max_total 50.0, got %v", state["max_total"])
	}
}

func TestDashboardTripAndSimulate(t *testing.T) {
	_, mux := setupTestDashboard(t)

	// Simulate a call
	simPayload := `{"agent_id": "test-bot", "target": "api.openai.com", "amount": 0.05, "action": "POST /v1/chat"}`
	req := httptest.NewRequest(http.MethodPost, "/bound/api/simulate", strings.NewReader(simPayload))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from simulate, got %d", rr.Code)
	}

	// Trip kill switch
	reqTrip := httptest.NewRequest(http.MethodPost, "/bound/api/trip", nil)
	rrTrip := httptest.NewRecorder()
	mux.ServeHTTP(rrTrip, reqTrip)

	if rrTrip.Code != http.StatusOK {
		t.Fatalf("expected 200 from trip, got %d", rrTrip.Code)
	}

	// Simulate another call - should now be blocked!
	reqBlocked := httptest.NewRequest(http.MethodPost, "/bound/api/simulate", strings.NewReader(simPayload))
	rrBlocked := httptest.NewRecorder()
	mux.ServeHTTP(rrBlocked, reqBlocked)

	var simResult map[string]interface{}
	_ = json.Unmarshal(rrBlocked.Body.Bytes(), &simResult)
	if simResult["allowed"] == true {
		t.Errorf("expected call to be blocked while tripped, but was allowed")
	}
}
