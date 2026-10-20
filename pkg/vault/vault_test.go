package vault

import (
	"path/filepath"
	"testing"
)

func TestVaultLimits(t *testing.T) {
	tempDir := t.TempDir()
	store := filepath.Join(tempDir, "vault.json")

	policy := Policy{
		MaxTotal:            20.00,
		MaxDaily:            15.00,
		MaxHourly:           10.00,
		MaxPerRequest:       8.00,
		RequireConfirmAbove: 6.00,
	}

	v := NewVault(policy, store)

	// 1. Normal allowed spend
	res := v.Check("agent-1", "api.openai.com", 2.00)
	if !res.Allowed {
		t.Fatalf("expected 2.00 spend to be allowed, got: %s", res.Reason)
	}
	if res.RequiresConfirm {
		t.Errorf("expected 2.00 spend to not require confirm")
	}

	_ = v.Record(Transaction{AgentID: "agent-1", Amount: 2.00, Target: "api.openai.com"})

	// 2. Human confirm threshold ($2 spent + $7 = $9 <= $10 max hourly, but $7 >= $6 confirm threshold)
	res = v.Check("agent-1", "aws.com", 7.00)
	if !res.Allowed {
		t.Fatalf("expected 7.00 spend to be allowed, got: %s", res.Reason)
	}
	if !res.RequiresConfirm {
		t.Errorf("expected 7.00 spend to require human confirm")
	}

	// 3. Single request limit exceeded ($9.50 > $8.00 MaxPerRequest)
	res = v.Check("agent-1", "aws.com", 9.50)
	if res.Allowed {
		t.Fatalf("expected 9.50 spend to exceed single limit of 8.00")
	}

	// 4. Hourly velocity exceeded ($2 spent + $9.00 = $11.00 > $10.00 max hourly)
	res = v.Check("agent-1", "api.anthropic.com", 9.00)
	if res.Allowed {
		t.Fatalf("expected 9.00 spend to exceed hourly limit of 10.00 (current: 2.00)")
	}

	// 5. Check remaining balance ($20 max - $2 spent = $18 remaining)
	rem := v.Remaining()
	if rem != 18.00 {
		t.Errorf("expected remaining budget 18.00, got %.2f", rem)
	}
}
