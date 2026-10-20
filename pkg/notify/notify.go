package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type EscrowRequest struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Target    string    `json:"target"`
	Amount    float64   `json:"amount"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

type Notifier struct {
	WebhookURL string
	client     *http.Client
}

func NewNotifier(webhookURL string) *Notifier {
	return &Notifier{
		WebhookURL: webhookURL,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

// SendAlert pushes an immediate notification to the configured webhook channel.
func (n *Notifier) SendAlert(req EscrowRequest) error {
	if n.WebhookURL == "" {
		return nil
	}

	payload := map[string]interface{}{
		"text": fmt.Sprintf("⚠️ [BOUND ALERT] Agent `%s` requests approval for **$%.2f** at `%s`.\nReason: %s",
			req.AgentID, req.Amount, req.Target, req.Reason),
		"agent_id":  req.AgentID,
		"amount":    req.Amount,
		"target":    req.Target,
		"timestamp": req.Timestamp.Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", n.WebhookURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
