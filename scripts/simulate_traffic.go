package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type SimulationAgent struct {
	ID        string
	Role      string
	Target    string
	AvgTokens int
	Risk      string
}

func main() {
	proxyURL := "http://localhost:8443/v1/chat/completions"
	if len(os.Args) > 1 {
		proxyURL = os.Args[1]
	}

	fmt.Println("================================================================")
	fmt.Println("   BOUND TRAFFIC GENERATOR // AUTONOMOUS AGENT SIMULATOR        ")
	fmt.Println("================================================================")
	fmt.Printf("Directing agent HTTP traffic to: %s\n\n", proxyURL)

	agents := []SimulationAgent{
		{ID: "research-agent-01", Role: "Market Researcher", Target: "https://api.openai.com/v1", AvgTokens: 800, Risk: "low"},
		{ID: "code-debugger-04", Role: "Self-Healing Bugfixer", Target: "https://api.anthropic.com/v1", AvgTokens: 2500, Risk: "medium"},
		{ID: "finance-executor-09", Role: "Autonomous Stripe Trader", Target: "https://api.stripe.com/v1", AvgTokens: 12000, Risk: "high"},
		{ID: "runaway-crawler-99", Role: "Scraper Loop", Target: "https://api.openai.com/v1", AvgTokens: 400, Risk: "loop"},
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for round := 1; round <= 20; round++ {
		fmt.Printf("\n--- [SIMULATION ROUND %02d] ---\n", round)

		for _, ag := range agents {
			tokens := ag.AvgTokens + rand.Intn(300)
			model := "gpt-4o"
			if ag.Risk == "medium" {
				model = "claude-3-5-sonnet-latest"
			} else if ag.Risk == "high" {
				model = "gpt-4o"
				tokens = 15000 // trigger high dollar cost to trip escrow
			}

			// Runaway loop agent sends identical payload repeatedly
			promptText := fmt.Sprintf("Execute step %d for role %s", round, ag.Role)
			if ag.Risk == "loop" {
				promptText = "STATIC_RETRY_ACTION_RECURSIVE_FAIL"
			}

			payload := map[string]interface{}{
				"model": model,
				"messages": []map[string]string{
					{"role": "user", "content": promptText},
				},
				"max_tokens": tokens,
			}
			data, _ := json.Marshal(payload)

			req, err := http.NewRequest("POST", proxyURL, bytes.NewReader(data))
			if err != nil {
				continue
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Bound-Agent", ag.ID)
			req.Header.Set("X-Bound-Reason", fmt.Sprintf("Autonomous execution: %s", ag.Role))

			start := time.Now()
			resp, err := client.Do(req)
			elapsed := time.Since(start)

			if err != nil {
				fmt.Printf("❌ [%s] Proxy unreachable (%v)\n", ag.ID, err)
				continue
			}

			var body map[string]interface{}
			_ = json.NewDecoder(resp.Body).Decode(&body)
			_ = resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
				fmt.Printf("✅ [%s] 200 OK | Cost approved | Latency: %v\n", ag.ID, elapsed)
			case http.StatusPaymentRequired:
				fmt.Printf("🛑 [%s] 402 PAYMENT REQUIRED | Spend cap / velocity triggered: %v\n", ag.ID, body["error"])
			case http.StatusForbidden:
				fmt.Printf("🔒 [%s] 403 FORBIDDEN | Circuit Breaker Tripped: %v\n", ag.ID, body["error"])
			case http.StatusAccepted:
				fmt.Printf("⏳ [%s] 202 ACCEPTED | Placed in Escrow for human review: %v\n", ag.ID, body["message"])
			default:
				fmt.Printf("⚠️ [%s] %d %s\n", ag.ID, resp.StatusCode, resp.Status)
			}

			time.Sleep(200 * time.Millisecond)
		}

		time.Sleep(1 * time.Second)
	}

	fmt.Println("\nSimulation cycle complete. Check dashboard at http://localhost:8443/dashboard")
}
