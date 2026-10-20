package governor

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/404kernel/bound/pkg/vault"
)

var (
	ErrTargetBlocked      = errors.New("target endpoint or tool is not in allowlist")
	ErrRunawayLoopTripped = errors.New("circuit breaker tripped: runaway agent loop detected")
)

type Config struct {
	AllowDomains       []string `json:"allow_domains"`
	BlockDomains       []string `json:"block_domains"`
	MaxLoopRepeats     int      `json:"max_loop_repeats"`      // Trip breaker if identical call > N times
	MaxCallsPerMinute  int      `json:"max_calls_per_minute"`  // Max calls per minute
}

func DefaultConfig() Config {
	return Config{
		AllowDomains:      []string{"api.openai.com", "api.anthropic.com", "generativelanguage.googleapis.com", "api.github.com", "api.stripe.com"},
		BlockDomains:      []string{"raw.githubusercontent.com/malicious", "pastebin.com"},
		MaxLoopRepeats:    5,
		MaxCallsPerMinute: 60,
	}
}

// ModelPricing rates in USD per 1,000 tokens
var DefaultPricing = map[string]struct{ Input, Output float64 }{
	// OpenAI
	"gpt-4o":                   {Input: 0.0025, Output: 0.010},
	"gpt-4o-mini":              {Input: 0.00015, Output: 0.0006},
	"gpt-4-turbo":              {Input: 0.0100, Output: 0.030},
	"o1":                       {Input: 0.0150, Output: 0.060},
	"o1-mini":                  {Input: 0.0030, Output: 0.012},
	"o3-mini":                  {Input: 0.0011, Output: 0.0044},
	// Anthropic
	"claude-3-5-sonnet-latest": {Input: 0.0030, Output: 0.015},
	"claude-3-5-sonnet":        {Input: 0.0030, Output: 0.015},
	"claude-3-7-sonnet":        {Input: 0.0030, Output: 0.015},
	"claude-3-5-haiku-latest":  {Input: 0.0008, Output: 0.004},
	"claude-3-5-haiku":         {Input: 0.0008, Output: 0.004},
	"claude-3-opus-latest":     {Input: 0.0150, Output: 0.075},
	// Google Gemini
	"gemini-2.0-flash":         {Input: 0.0001, Output: 0.0004},
	"gemini-2.5-flash":         {Input: 0.0001, Output: 0.0004},
	"gemini-1.5-pro":           {Input: 0.00125, Output: 0.0050},
	"gemini-1.5-flash":         {Input: 0.000075, Output: 0.0003},
	// DeepSeek
	"deepseek-chat":            {Input: 0.00014, Output: 0.00028},
	"deepseek-reasoner":        {Input: 0.00055, Output: 0.00219},
	// Fallback Default
	"default":                  {Input: 0.0010, Output: 0.0020},
}

type CallFingerprint struct {
	Target  string
	Payload string
	Count   int
	LastAt  time.Time
}

type Governor struct {
	mu            sync.Mutex
	cfg           Config
	v             *vault.Vault
	recentCalls   []time.Time
	fingerprints  map[string]*CallFingerprint
	BreakerTripped bool
	TripReason    string
}

func NewGovernor(cfg Config, v *vault.Vault) *Governor {
	return &Governor{
		cfg:          cfg,
		v:            v,
		fingerprints: make(map[string]*CallFingerprint),
	}
}

// EstimateTokenCost calculates expected cost in USD for token usage.
func EstimateTokenCost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := DefaultPricing[strings.ToLower(model)]
	if !ok {
		pricing = DefaultPricing["default"]
	}

	cost := (float64(inputTokens) / 1000.0 * pricing.Input) + (float64(outputTokens) / 1000.0 * pricing.Output)
	return cost
}

type Decision struct {
	Allowed         bool
	RequiresConfirm bool
	Reason          string
	Cost            float64
}

// Evaluate performs a security and budget assessment before any tool or API call executes.
func (g *Governor) Evaluate(agentID, target, payloadHash string, estimatedCost float64) Decision {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 1. Check if Circuit Breaker is tripped
	if g.BreakerTripped {
		return Decision{
			Allowed: false,
			Reason:  fmt.Sprintf("Circuit breaker active: %s", g.TripReason),
		}
	}

	now := time.Now()

	// 2. Rate Limit & Runaway Loop Detection
	// Clean calls older than 1 minute
	var validCalls []time.Time
	for _, t := range g.recentCalls {
		if now.Sub(t) < time.Minute {
			validCalls = append(validCalls, t)
		}
	}
	g.recentCalls = append(validCalls, now)

	if g.cfg.MaxCallsPerMinute > 0 && len(g.recentCalls) > g.cfg.MaxCallsPerMinute {
		g.BreakerTripped = true
		g.TripReason = fmt.Sprintf("Exceeded %d calls per minute limit", g.cfg.MaxCallsPerMinute)
		return Decision{Allowed: false, Reason: g.TripReason}
	}

	// 3. Detect Identical Tool Call Loops (Infinite recursion bug)
	if payloadHash != "" {
		fp, exists := g.fingerprints[payloadHash]
		if !exists {
			g.fingerprints[payloadHash] = &CallFingerprint{
				Target:  target,
				Payload: payloadHash,
				Count:   1,
				LastAt:  now,
			}
		} else {
			if now.Sub(fp.LastAt) < 30*time.Second {
				fp.Count++
				fp.LastAt = now
				if g.cfg.MaxLoopRepeats > 0 && fp.Count > g.cfg.MaxLoopRepeats {
					g.BreakerTripped = true
					g.TripReason = fmt.Sprintf("Runaway loop detected: identical action executed %d times in 30s", fp.Count)
					return Decision{Allowed: false, Reason: g.TripReason}
				}
			} else {
				fp.Count = 1
				fp.LastAt = now
			}
		}
	}

	// 4. Domain / Target Allowlist & Blocklist
	if g.isBlocked(target) {
		return Decision{
			Allowed: false,
			Reason:  fmt.Sprintf("Target '%s' is blocked by security policy", target),
		}
	}

	// 5. Vault Financial Authorization Check
	check := g.v.Check(agentID, target, estimatedCost)
	if !check.Allowed {
		return Decision{
			Allowed: false,
			Cost:    estimatedCost,
			Reason:  check.Reason,
		}
	}

	return Decision{
		Allowed:         true,
		RequiresConfirm: check.RequiresConfirm,
		Cost:            estimatedCost,
		Reason:          "Authorized by Bound Governor",
	}
}

func (g *Governor) isBlocked(target string) bool {
	lower := strings.ToLower(target)
	for _, b := range g.cfg.BlockDomains {
		if strings.Contains(lower, strings.ToLower(b)) {
			return true
		}
	}
	return false
}

func (g *Governor) ResetBreaker() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.BreakerTripped = false
	g.TripReason = ""
	g.fingerprints = make(map[string]*CallFingerprint)
	g.recentCalls = nil
}

func (g *Governor) Trip(reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.BreakerTripped = true
	if reason == "" {
		reason = "Emergency manual kill-switch engaged"
	}
	g.TripReason = reason
}

func (g *Governor) Status() (bool, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.BreakerTripped, g.TripReason
}
