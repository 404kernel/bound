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
	WebhookURL     string
	TelegramBot    string
	TelegramChatID string
	client         *http.Client
}

func NewNotifier(webhookURL, telegramBot, telegramChatID string) *Notifier {
	return &Notifier{
		WebhookURL:     webhookURL,
		TelegramBot:    telegramBot,
		TelegramChatID: telegramChatID,
		client:         &http.Client{Timeout: 5 * time.Second},
	}
}

// SendAlert pushes an immediate notification to the configured webhook channel and Telegram.
func (n *Notifier) SendAlert(req EscrowRequest) error {
	var lastErr error

	// 1. Generic / Slack / Discord Webhook
	if n.WebhookURL != "" {
		payload := map[string]interface{}{
			"text": fmt.Sprintf("[BOUND ESCROW] Agent '%s' requests approval for $%.2f at %s.\nReason: %s",
				req.AgentID, req.Amount, req.Target, req.Reason),
			"agent_id":  req.AgentID,
			"amount":    req.Amount,
			"target":    req.Target,
			"timestamp": req.Timestamp.Format(time.RFC3339),
		}

		data, _ := json.Marshal(payload)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", n.WebhookURL, bytes.NewReader(data))
		if err == nil {
			httpReq.Header.Set("Content-Type", "application/json")
			resp, err := n.client.Do(httpReq)
			if err == nil {
				_ = resp.Body.Close()
			} else {
				lastErr = err
			}
		}
		cancel()
	}

	// 2. Telegram Bot API Direct Alert
	if n.TelegramBot != "" && n.TelegramChatID != "" {
		tgURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.TelegramBot)
		text := fmt.Sprintf("🔒 *BOUND ESCROW HOLD*\n\n*Agent:* `%s`\n*Target:* `%s`\n*Amount:* `$%.2f`\n*Reason:* %s\n\n_Review in Bound Dashboard at http://localhost:8443/dashboard_",
			req.AgentID, req.Target, req.Amount, req.Reason)

		// Telegram inline keyboard for one-tap mobile decision
		type InlineButton struct {
			Text         string `json:"text"`
			CallbackData string `json:"callback_data"`
		}
		type InlineKeyboard struct {
			InlineKeyboard [][]InlineButton `json:"inline_keyboard"`
		}

		keyboard := InlineKeyboard{
			InlineKeyboard: [][]InlineButton{
				{
					{Text: fmt.Sprintf("✅ Approve ($%.2f)", req.Amount), CallbackData: fmt.Sprintf("bound_approve:%s", req.AgentID)},
					{Text: "❌ Reject & Halt", CallbackData: fmt.Sprintf("bound_reject:%s", req.AgentID)},
				},
			},
		}

		payload := map[string]interface{}{
			"chat_id":      n.TelegramChatID,
			"text":         text,
			"parse_mode":   "Markdown",
			"reply_markup": keyboard,
		}
		data, _ := json.Marshal(payload)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		httpReq, err := http.NewRequestWithContext(ctx, "POST", tgURL, bytes.NewReader(data))
		if err == nil {
			httpReq.Header.Set("Content-Type", "application/json")
			resp, err := n.client.Do(httpReq)
			if err == nil {
				_ = resp.Body.Close()
			} else {
				lastErr = err
			}
		}
	}

	return lastErr
}
