<div align="center">

```
  ██████╗  ██████╗ ██╗   ██╗███╗   ██╗██████╗ 
  ██╔══██╗██╔═══██╗██║   ██║████╗  ██║██╔══██╗
  ██████╔╝██║   ██║██║   ██║██╔██╗ ██║██║  ██║
  ██╔══██╗██║   ██║██║   ██║██║╚██╗██║██║  ██║
  ██████╔╝╚██████╔╝╚██████╔╝██║ ╚████║██████╔╝
  ╚═════╝  ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝╚═════╝ 
```

### ⚡ The Financial & Execution Governor for Autonomous AI Agents

[![Go Version](https://img.shields.io/badge/Go-1.27+-7D56F4.svg?style=for-the-badge&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-04B575.svg?style=for-the-badge)](https://opensource.org/licenses/MIT)
[![Latency](https://img.shields.io/badge/Overhead-<2ms-00D7FE.svg?style=for-the-badge)](https://github.com/404kernel/bound)
[![Architecture](https://img.shields.io/badge/Zero--Trust-Enforced-FFB86C.svg?style=for-the-badge)](https://github.com/404kernel/bound)

<br/>

**Eliminate the anxiety of letting autonomous LLMs touch money, APIs, and execution rails.**

[Quick Start](#-quick-start) • [The Problem](#-the-problem) • [Architecture](#-architecture) • [CLI Commands](#-cli-commands) • [SDK Integration](#-sdk-integration)

<br/>

```text
╭──────────────────────────────────────────────────────────╮
│ 📊 BOUND GOVERNANCE TELEMETRY & VAULT STATUS            │
├──────────────────────────────────────────────────────────┤
│  Total Spent       : $1.4230                             │
│  Remaining Budget  : $23.5770                            │
│  Max Lifetime Cap  : $25.00                              │
│  Daily Limit       : $10.00                              │
│  Hourly Velocity   : $5.00                               │
│  Total Transactions: 14                                  │
│  Circuit Breaker   : 🟢 ACTIVE                           │
╰──────────────────────────────────────────────────────────╯
```

</div>

---

## 🛑 The Problem

Autonomous AI agents (Claude Code, AutoGPT, CrewAI, LangChain, custom loops) are powerful, but they operate with **unrestricted, blank-check authority**:

1. **Runaway Loop Bankruptcy**: An infinite planning or debugging loop can burn $1,000+ in OpenAI or Anthropic tokens before you wake up.
2. **The Credit Card Dilemma**: You cannot safely hand raw API keys or credit card numbers to an LLM context. One prompt injection can hijack the credentials or spend them.
3. **No Vendor-Neutral Leash**: Cloud providers and payment processors have zero incentive to cap your spend.

---

## 🛡️ What is BOUND?

**Bound** is a blazing-fast, single-binary proxy daemon written in pure Go. It sits between your AI agent and the outside world, acting as an **intelligent financial governor and circuit breaker**:

- 🔒 **Spend Vaults**: Strict lifetime, daily, and hourly velocity limits per agent.
- ⚡ **Circuit Breakers**: Detects runaway loops (>5 identical tool calls in 30s) and cuts connection instantly.
- 🛑 **Human-in-the-Loop Escrow**: Automatically pauses the agent and sends an approval request (Telegram, Slack, Discord) for transactions exceeding a threshold (e.g. > $15).
- 📜 **Cryptographic Ledger**: Tamper-proof JSONL audit trail recording every prompt, tool call, dollar spent, and latency telemetry.
- 🏎️ **Sub-2ms Latency**: Zero garbage collection lag. Written in pure Go with zero runtime dependencies.

---

## 🏛️ Architecture

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

## 🚀 Quick Start

### Installation

```bash
# Clone and build in 5 seconds
git clone https://github.com/404kernel/bound.git
cd bound
go build -o bound.exe ./cmd/bound
```

### Option 1: Wrap ANY Agent in One Command

Run any existing Python, Node, or Go agent script wrapped in a strict $5.00 budget:

```bash
bound run --budget 5.00 --hourly 2.00 -- python my_agent.py
```

*Bound automatically starts a local proxy, injects environment variables (`HTTP_PROXY`, `OPENAI_BASE_URL`), and cuts the process if the agent attempts to exceed $5.00.*

### Option 2: Start the Standalone Proxy Daemon

```bash
bound proxy --port :8443 --budget 50.00 --hourly 10.00 --upstream https://api.openai.com
```

---

## 🎮 CLI Commands

| Command | Action |
|---|---|
| `bound proxy` | Starts the standalone HTTP/HTTPS governance daemon |
| `bound run --budget <amt> -- <cmd>` | Executes an agent process wrapped in real-time spend limits |
| `bound status` | Renders the live ASCII budget, velocity, and circuit breaker dashboard |
| `bound audit` | Streams the cryptographic decision and transaction audit ledger |
| `bound reset` | Clears vault balances and resets tripped circuit breakers |
| `bound help` | Displays the interactive CLI reference guide |

---

## 💻 SDK Integration

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

## 📜 License

MIT License © 2026 404kernel
