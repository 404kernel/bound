"""
Example: Running an OpenAI Agent behind the Bound Governor Proxy.

All requests flow through Bound (http://localhost:8443) which tracks token spend,
enforces velocity limits, and triggers Telegram / webhook alerts if limits are hit.
"""

import os
from openai import OpenAI

# 1. Direct OpenAI client to the local Bound proxy
client = OpenAI(
    base_url="http://localhost:8443/v1",
    api_key=os.environ.get("OPENAI_API_KEY", "sk-mock-key-bound"),
    default_headers={
        "X-Bound-Agent": "crawler-agent-01",
        "X-Bound-Reason": "Market data extraction loop",
    }
)

def run_agent_task(prompt: str):
    print(f"[Agent] Sending prompt to OpenAI through Bound proxy...")
    try:
        response = client.chat.completions.create(
            model="gpt-4o-mini",
            messages=[
                {"role": "system", "content": "You are a concise research agent."},
                {"role": "user", "content": prompt}
            ],
            max_tokens=150,
        )
        print(f"[Agent] Response received: {response.choices[0].message.content.strip()}")
    except Exception as e:
        print(f"[Bound Intercepted] Request rejected or paused: {e}")

if __name__ == "__main__":
    run_agent_task("Summarize the benefits of financial spend vaults for AI agents in one sentence.")
