package governor

import (
	"path/filepath"
	"testing"

	"github.com/404kernel/bound/pkg/vault"
)

func TestGovernorRunawayLoop(t *testing.T) {
	tempDir := t.TempDir()
	v := vault.NewVault(vault.DefaultPolicy(), filepath.Join(tempDir, "v.json"))

	cfg := DefaultConfig()
	cfg.MaxLoopRepeats = 3 // small threshold to test breaker
	gov := NewGovernor(cfg, v)

	// Send identical payload 4 times
	hash := "search_web:golang_concurrency"
	for i := 1; i <= 3; i++ {
		dec := gov.Evaluate("agent-1", "api.openai.com", hash, 0.001)
		if !dec.Allowed {
			t.Fatalf("call %d expected allowed, got: %s", i, dec.Reason)
		}
	}

	// 4th call should trip circuit breaker
	dec := gov.Evaluate("agent-1", "api.openai.com", hash, 0.001)
	if dec.Allowed {
		t.Fatalf("expected 4th identical call to trip circuit breaker")
	}
	if !gov.BreakerTripped {
		t.Errorf("expected BreakerTripped to be true")
	}
}

func TestEstimateTokenCost(t *testing.T) {
	cost := EstimateTokenCost("gpt-4o", 1000, 1000)
	// gpt-4o: $0.0025 + $0.010 = $0.0125
	expected := 0.0125
	if cost != expected {
		t.Errorf("expected cost %.4f, got %.4f", expected, cost)
	}
}
