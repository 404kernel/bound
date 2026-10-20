<p align="center">
  <a href="https://github.com/404kernel/bound">
    <img src="assets/icon.png" width="120" height="120" alt="Bound Logo" />
  </a>
</p>

<h1 align="center">Bound</h1>

<p align="center">
  <strong>Programmable spend vaults and financial rails for autonomous AI agents.</strong>
</p>

<p align="center">
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.27+-7D56F4.svg?style=flat-square&logo=go" alt="Go Version" /></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-04B575.svg?style=flat-square" alt="MIT License" /></a>
  <a href="https://github.com/404kernel/bound"><img src="https://img.shields.io/badge/Latency-<2ms-00D7FE.svg?style=flat-square" alt="Sub-2ms" /></a>
  <a href="https://github.com/404kernel/bound"><img src="https://img.shields.io/badge/Zero--Trust-Enforced-FFB86C.svg?style=flat-square" alt="Zero Trust" /></a>
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> •
  <a href="#the-problem">The Problem</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#cli-reference">CLI Reference</a> •
  <a href="#sdk-integration">SDK Integration</a>
</p>

```text
╭──────────────────────────────────────────────────────────╮
│ BOUND GOVERNANCE TELEMETRY & VAULT STATUS                │
├──────────────────────────────────────────────────────────┤
│  Total Spent       : $1.4230                             │
│  Remaining Budget  : $23.5770                            │
│  Max Lifetime Cap  : $25.00                              │
│  Daily Limit       : $10.00                              │
│  Hourly Velocity   : $5.00                               │
│  Total Transactions: 14                                  │
│  Circuit Breaker   : ACTIVE                              │
╰──────────────────────────────────────────────────────────╯
```

---

## The Problem

Autonomous AI agents (Claude Code, AutoGPT, CrewAI, LangChain, custom loops) are powerful, but they operate with **unrestricted, blank-check authority**:

1. **Runaway Loop Bankruptcy**: An infinite planning or debugging loop can burn $1,000+ in OpenAI or Anthropic tokens before you wake up.
2. **The Credit Card Dilemma**: You cannot safely hand raw API keys or credit card numbers to an LLM context. One prompt injection can hijack the credentials or spend them.
3. **No Vendor-Neutral Leash**: Cloud providers and payment processors have zero incentive to cap your spend.

---

## Overview

**Bound** is a blazing-fast, single-binary proxy daemon written in pure Go. It sits between your AI agent and the outside world, acting as an **intelligent financial governor and circuit breaker**:

- **Spend Vaults**: Strict lifetime, daily, and hourly velocity limits per agent.
- **Circuit Breakers**: Detects runaway loops (>5 identical tool calls in 30s) and cuts connection instantly.
- **Human-in-the-Loop Escrow**: Automatically pauses the agent and sends an approval request (Telegram, Slack, Discord) for transactions exceeding a threshold (e.g. > $15).
- **Cryptographic Ledger**: Tamper-proof JSONL audit trail recording every prompt, tool call, dollar spent, and latency telemetry.
- **Sub-2ms Latency**: Zero garbage collection lag. Written in pure Go with zero runtime dependencies.

---

## Architecture

```
                                      BOUND GOVERNANCE PROXY
                                 ┌────────────────────────────────┐
┌──────────────┐                 │  1. Circuit Breaker Inspection │                 ┌─────────────────┐
│              │   HTTP Proxy    │     (Runaway Loop Detection)   │   HTTPS Upstream│  OpenAI / AWS   │
│  Your Agent  │ ──────────────> │                                │ ──────────────> │  Stripe / Cloud │
│ (Python/Go)  │ <────────────── │  2. Budget Vault Authorization │ <────────────── │  GitHub / APIs  │
└──────────────┘   X-Bound-Status│     (Daily/Hourly Spend Caps)  │                 └─────────────────┘
                                 │                                │
                                 │  3. Human Escrow / Webhooks    │
                                 │     (Telegram / Slack Alert)   │
                                 └────────────────────────────────┘
                                                 │
                                                 ▼
                                        ~/.bound/audit.jsonl
```

---

## Quick Start

### 1. One-Line Instant Installation

**macOS & Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/404kernel/bound/main/install.sh | sh
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/404kernel/bound/main/install.ps1 | iex
```

**Homebrew (macOS):**
```bash
brew tap 404kernel/bound
brew install bound
```

**Python SDK:**
```bash
pip install bound-ai
```

---

### 2. Configure Alerts in 30 Seconds

Run the interactive setup wizard to link Telegram, Discord, or Slack:
```bash
bound setup
```

---

### 3. Usage Modes

#### Option A: Wrap ANY Agent Process in One Command
Run any Python, Node, or Go agent wrapped in strict budget caps:
```bash
bound run --budget 5.00 --hourly 2.00 -- python my_agent.py
```

#### Option B: Start the Standalone Proxy Daemon
```bash
bound proxy --port :8443 --budget 50.00 --hourly 10.00
```

#### Option C: Live Multi-Agent Simulation
Test velocity limits, runaway loop circuit breakers, and Telegram escrows in real-time:
```bash
go run ./scripts/simulate_traffic.go
```

### Option 3: Real-Time Web Dashboard

Whenever the proxy is running, navigate to:
```text
http://localhost:8443/dashboard
```

Features:
- **Live Stream**: Auto-refreshing table of intercepted agent calls and costs
- **Spend Gauges**: Real-time visualization of daily caps and velocity limits
- **Emergency Kill Switch**: One-click freeze to instantly disarm all outbound agent traffic
- **Agent Pools**: Per-agent spend breakdown and active audit records

---

## CLI Reference

| Command | Action |
|---|---|
| `bound proxy` | Starts the standalone HTTP/HTTPS governance daemon |
| `bound run --budget <amt> -- <cmd>` | Executes an agent process wrapped in real-time spend limits |
| `bound status` | Renders the live ASCII budget, velocity, and circuit breaker dashboard |
| `bound audit` | Streams the cryptographic decision and transaction audit ledger |
| `bound reset` | Clears vault balances and resets tripped circuit breakers |
| `bound help` | Displays the interactive CLI reference guide |

---

## SDK Integration

### Python
```python
import os

# Point any standard LLM SDK to Bound's local governor
os.environ["OPENAI_BASE_URL"] = "http://localhost:8443/v1"
os.environ["HTTP_PROXY"] = "http://localhost:8443"

from openai import OpenAI
client = OpenAI() # Now 100% governed by Bound
```

### Go
```go
package main

import (
    "github.com/404kernel/bound/pkg/vault"
    "github.com/404kernel/bound/pkg/governor"
)

func main() {
    v := vault.NewVault(vault.DefaultPolicy(), "")
    gov := governor.NewGovernor(governor.DefaultConfig(), v)

    // Evaluate proposed tool call or spend before execution
    decision := gov.Evaluate("agent-01", "api.stripe.com", "charge_customer", 12.50)
    if !decision.Allowed {
        panic(decision.Reason) // Blocked by Bound
    }
}
```

---

## License

MIT License © 2026 404kernel
