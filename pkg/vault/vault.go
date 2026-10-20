package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrBudgetExceeded    = errors.New("budget limit exceeded")
	ErrHourlyCapExceeded = errors.New("hourly spend velocity limit exceeded")
	ErrDailyCapExceeded  = errors.New("daily spend limit exceeded")
	ErrSingleCapExceeded = errors.New("single request cost cap exceeded")
	ErrApprovalRequired  = errors.New("human-in-the-loop approval required")
)

// Policy defines strict financial and execution boundaries for an agent or team pool.
type Policy struct {
	MaxTotal            float64 `json:"max_total"`             // Hard lifetime budget (e.g. 50.00)
	MaxDaily            float64 `json:"max_daily"`             // Rolling 24h budget (e.g. 20.00)
	MaxHourly           float64 `json:"max_hourly"`            // Velocity limit to kill runaway loops (e.g. 5.00)
	MaxPerRequest       float64 `json:"max_per_request"`       // Cap per single tool call / transaction (e.g. 10.00)
	RequireConfirmAbove float64 `json:"require_confirm_above"` // Pause & require approval if above this (e.g. 15.00)
}

// DefaultPolicy provides safe out-of-the-box defaults ($25 total, $10/day, $5/hr).
func DefaultPolicy() Policy {
	return Policy{
		MaxTotal:            25.00,
		MaxDaily:            10.00,
		MaxHourly:           5.00,
		MaxPerRequest:       10.00,
		RequireConfirmAbove: 15.00,
	}
}

// Transaction represents an individual spend event.
type Transaction struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	Target      string    `json:"target"`
	Amount      float64   `json:"amount"`
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
	Approved    bool      `json:"approved"`
}

// Vault manages thread-safe balance tracking and limits enforcement.
type Vault struct {
	mu           sync.RWMutex
	Policy       Policy                 `json:"policy"`
	TotalSpent   float64                `json:"total_spent"`
	Transactions []Transaction          `json:"transactions"`
	AgentSpend   map[string]float64     `json:"agent_spend"`
	StoragePath  string                 `json:"-"`
}

func NewVault(p Policy, storagePath string) *Vault {
	if storagePath == "" {
		home, _ := os.UserHomeDir()
		storagePath = filepath.Join(home, ".bound", "vault.json")
	}

	v := &Vault{
		Policy:      p,
		AgentSpend:  make(map[string]float64),
		StoragePath: storagePath,
	}

	_ = v.Load()
	return v
}

type CheckResult struct {
	Allowed         bool
	RequiresConfirm bool
	Reason          string
	CurrentTotal    float64
	RemainingTotal  float64
}

// Check evaluates whether a proposed spend is permissible under the policy.
func (v *Vault) Check(agentID string, target string, amount float64) CheckResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// 1. Check single request cap
	if v.Policy.MaxPerRequest > 0 && amount > v.Policy.MaxPerRequest {
		return CheckResult{
			Allowed:        false,
			Reason:         fmt.Sprintf("Request cost $%.4f exceeds max per-request limit $%.2f", amount, v.Policy.MaxPerRequest),
			CurrentTotal:   v.TotalSpent,
			RemainingTotal: v.remainingTotal(),
		}
	}

	// 2. Check total lifetime budget
	if v.Policy.MaxTotal > 0 && (v.TotalSpent+amount) > v.Policy.MaxTotal {
		return CheckResult{
			Allowed:        false,
			Reason:         fmt.Sprintf("Spend $%.4f would exceed total budget $%.2f (Current: $%.2f)", amount, v.Policy.MaxTotal, v.TotalSpent),
			CurrentTotal:   v.TotalSpent,
			RemainingTotal: v.remainingTotal(),
		}
	}

	// 3. Check rolling hourly velocity limit
	hourly := v.spendSince(time.Now().Add(-1 * time.Hour))
	if v.Policy.MaxHourly > 0 && (hourly+amount) > v.Policy.MaxHourly {
		return CheckResult{
			Allowed:        false,
			Reason:         fmt.Sprintf("Spend $%.4f exceeds hourly velocity limit $%.2f (Spent last hr: $%.2f)", amount, v.Policy.MaxHourly, hourly),
			CurrentTotal:   v.TotalSpent,
			RemainingTotal: v.remainingTotal(),
		}
	}

	// 4. Check rolling 24-hour limit
	daily := v.spendSince(time.Now().Add(-24 * time.Hour))
	if v.Policy.MaxDaily > 0 && (daily+amount) > v.Policy.MaxDaily {
		return CheckResult{
			Allowed:        false,
			Reason:         fmt.Sprintf("Spend $%.4f exceeds daily limit $%.2f (Spent last 24h: $%.2f)", amount, v.Policy.MaxDaily, daily),
			CurrentTotal:   v.TotalSpent,
			RemainingTotal: v.remainingTotal(),
		}
	}

	// 5. Check if human escrow confirmation is triggered
	requiresConfirm := false
	if v.Policy.RequireConfirmAbove > 0 && amount >= v.Policy.RequireConfirmAbove {
		requiresConfirm = true
	}

	return CheckResult{
		Allowed:         true,
		RequiresConfirm: requiresConfirm,
		CurrentTotal:    v.TotalSpent,
		RemainingTotal:  v.remainingTotal(),
	}
}

// Record commits an approved transaction into the vault and persists state.
func (v *Vault) Record(tx Transaction) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if tx.Timestamp.IsZero() {
		tx.Timestamp = time.Now()
	}
	if tx.ID == "" {
		tx.ID = fmt.Sprintf("tx_%d", time.Now().UnixNano())
	}
	tx.Approved = true

	v.Transactions = append(v.Transactions, tx)
	v.TotalSpent += tx.Amount
	v.AgentSpend[tx.AgentID] += tx.Amount

	return v.saveUnsafe()
}

func (v *Vault) spendSince(since time.Time) float64 {
	var total float64
	for _, tx := range v.Transactions {
		if tx.Timestamp.After(since) && tx.Approved {
			total += tx.Amount
		}
	}
	return total
}

func (v *Vault) remainingTotal() float64 {
	if v.Policy.MaxTotal <= 0 {
		return -1 // unlimited
	}
	rem := v.Policy.MaxTotal - v.TotalSpent
	if rem < 0 {
		return 0
	}
	return rem
}

func (v *Vault) Remaining() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.remainingTotal()
}

func (v *Vault) Summary() map[string]interface{} {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return map[string]interface{}{
		"total_spent":        v.TotalSpent,
		"remaining_budget":  v.remainingTotal(),
		"max_total":         v.Policy.MaxTotal,
		"max_daily":         v.Policy.MaxDaily,
		"max_hourly":        v.Policy.MaxHourly,
		"total_transactions": len(v.Transactions),
		"active_agents":     len(v.AgentSpend),
	}
}

func (v *Vault) saveUnsafe() error {
	if v.StoragePath == "" {
		return nil
	}
	dir := filepath.Dir(v.StoragePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(v.StoragePath, data, 0644)
}

func (v *Vault) Load() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.StoragePath == "" {
		return nil
	}
	data, err := os.ReadFile(v.StoragePath)
	if err != nil {
		return err
	}
	var loaded Vault
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	v.TotalSpent = loaded.TotalSpent
	v.Transactions = loaded.Transactions
	if loaded.AgentSpend != nil {
		v.AgentSpend = loaded.AgentSpend
	} else {
		v.AgentSpend = make(map[string]float64)
	}
	return nil
}

func (v *Vault) Reset() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.TotalSpent = 0
	v.Transactions = nil
	v.AgentSpend = make(map[string]float64)
	return v.saveUnsafe()
}

func (v *Vault) SetPolicy(p Policy) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Policy = p
	return v.saveUnsafe()
}

func (v *Vault) AgentBreakdown() map[string]float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	res := make(map[string]float64)
	for k, val := range v.AgentSpend {
		res[k] = val
	}
	return res
}

func (v *Vault) RecentTransactions(limit int) []Transaction {
	v.mu.RLock()
	defer v.mu.RUnlock()
	n := len(v.Transactions)
	if limit <= 0 || n <= limit {
		res := make([]Transaction, n)
		for i, tx := range v.Transactions {
			res[n-1-i] = tx
		}
		return res
	}
	res := make([]Transaction, limit)
	for i := 0; i < limit; i++ {
		res[i] = v.Transactions[n-1-i]
	}
	return res
}
