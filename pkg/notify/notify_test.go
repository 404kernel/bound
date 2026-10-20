package notify

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNotifierSendAlertWebhook(t *testing.T) {
	var received int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewNotifier(server.URL, "", "")
	err := n.SendAlert(EscrowRequest{
		AgentID:   "agent-007",
		Amount:    25.50,
		Target:    "https://api.stripe.com",
		Reason:    "High value charge triggered",
		Timestamp: time.Now(),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&received) != 1 {
		t.Fatalf("expected 1 webhook call, got %d", received)
	}
}
