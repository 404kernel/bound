# Bound Python SDK (`bound-ai`)

Programmable spend vaults and financial rails for autonomous AI agents.

## Installation

```bash
pip install bound-ai
```

## Quick Start (1-Line Protection)

Wrap any Python agent loop (LangChain, AutoGPT, CrewAI, raw OpenAI) with strict budget caps and loop breakers:

```python
import bound
from openai import OpenAI

# 1-line protection: routes all LLM calls through local Bound proxy (localhost:8443)
bound.protect(agent_id="finance-agent")

client = OpenAI()
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Analyze quarterly filings"}]
)
# Automatically tracked, velocity-capped, and governed by Bound!
```

## Programmatic Tool & Action Check

Before charging a credit card or launching cloud instances:

```python
import bound

allowed, reason, details = bound.check(
    target="api.stripe.com",
    amount=18.50,
    action="charge_customer"
)

if not allowed:
    print(f"Action blocked: {reason}")
else:
    print(f"Action authorized. Remaining budget: ${details['remaining_budget']}")
```

## Status & Telemetry

```python
import bound

stats = bound.status()
print(f"Spent: ${stats['total_spent']} | Remaining: ${stats['remaining_budget']}")
```
