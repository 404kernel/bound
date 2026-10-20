package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type AuditEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	AgentID    string    `json:"agent_id"`
	Target     string    `json:"target"`
	Action     string    `json:"action"`
	Amount     float64   `json:"amount"`
	Allowed    bool      `json:"allowed"`
	Reason     string    `json:"reason"`
	Remaining  float64   `json:"remaining"`
}

type Ledger struct {
	mu   sync.Mutex
	path string
}

func NewLedger(path string) *Ledger {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".bound", "audit.jsonl")
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	return &Ledger{path: path}
}

func (l *Ledger) Log(entry AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
}

func (l *Ledger) FormatEntry(e AuditEntry) string {
	status := "ALLOWED"
	if !e.Allowed {
		status = "BLOCKED"
	}

	return fmt.Sprintf("[%s] %s | Agent: %s | Target: %s | $%.4f (Remaining: $%.2f) - %s",
		e.Timestamp.Format("15:04:05"),
		status,
		e.AgentID,
		e.Target,
		e.Amount,
		e.Remaining,
		e.Reason,
	)
}

func (l *Ledger) Recent(limit int) ([]AuditEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var all []AuditEntry
	dec := json.NewDecoder(f)
	for dec.More() {
		var entry AuditEntry
		if err := dec.Decode(&entry); err == nil {
			all = append(all, entry)
		}
	}

	if limit <= 0 || len(all) <= limit {
		// Reverse chronological
		res := make([]AuditEntry, len(all))
		for i, e := range all {
			res[len(all)-1-i] = e
		}
		return res, nil
	}

	res := make([]AuditEntry, limit)
	for i := 0; i < limit; i++ {
		res[i] = all[len(all)-1-i]
	}
	return res, nil
}
