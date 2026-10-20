import os
import json
import urllib.request
import urllib.error
from typing import Optional, Dict, Any, Tuple

DEFAULT_PROXY = "http://localhost:8443"

class BoundError(Exception):
    """Raised when an action is blocked or fails governance check."""
    pass

class BoundClient:
    def __init__(self, proxy_url: str = DEFAULT_PROXY, agent_id: str = "default-agent"):
        self.proxy_url = proxy_url.rstrip("/")
        self.agent_id = agent_id

    def protect(self, agent_id: Optional[str] = None) -> None:
        """
        Injects Bound financial governor into process environment variables.
        All standard OpenAI, Anthropic, LangChain, and requests traffic will
        route directly through the local Bound security rails.
        """
        if agent_id:
            self.agent_id = agent_id

        os.environ["HTTP_PROXY"] = self.proxy_url
        os.environ["HTTPS_PROXY"] = self.proxy_url
        os.environ["OPENAI_BASE_URL"] = f"{self.proxy_url}/v1"
        os.environ["ANTHROPIC_BASE_URL"] = self.proxy_url
        os.environ["BOUND_AGENT_ID"] = self.agent_id

    def check(self, target: str, amount: float, action: str = "custom_tool", commit: bool = True) -> Tuple[bool, str, Dict[str, Any]]:
        """
        Evaluates a proposed tool call, credit card charge, or API action.
        Returns (allowed: bool, reason: str, details: dict).
        """
        url = f"{self.proxy_url}/bound/check"
        payload = json.dumps({
            "agent_id": self.agent_id,
            "target": target,
            "amount": amount,
            "action": action,
            "commit": commit
        }).encode("utf-8")

        req = urllib.request.Request(url, data=payload, headers={"Content-Type": "application/json"}, method="POST")

        try:
            with urllib.request.urlopen(req, timeout=5) as resp:
                data = json.loads(resp.read().decode("utf-8"))
                return data.get("allowed", False), data.get("reason", "Approved"), data
        except urllib.error.HTTPError as e:
            try:
                err_data = json.loads(e.read().decode("utf-8"))
                return False, err_data.get("reason", "Rejected by policy"), err_data
            except Exception:
                return False, f"HTTP Error {e.code}", {}
        except Exception as e:
            return False, f"Connection failed to Bound proxy: {str(e)}", {}

    def status(self) -> Dict[str, Any]:
        """Fetches current vault telemetry, remaining balance, and breaker status."""
        url = f"{self.proxy_url}/bound/status"
        req = urllib.request.Request(url, method="GET")
        try:
            with urllib.request.urlopen(req, timeout=5) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except Exception as e:
            return {"error": f"Failed to connect to Bound proxy: {str(e)}"}

    def reset(self) -> bool:
        """Resets vault spend totals and clears tripped circuit breaker."""
        url = f"{self.proxy_url}/bound/reset"
        req = urllib.request.Request(url, method="POST")
        try:
            with urllib.request.urlopen(req, timeout=5) as resp:
                return resp.status == 200
        except Exception:
            return False

# Global convenience instance
_default_client = BoundClient()

def protect(agent_id: str = "default-agent", proxy_url: str = DEFAULT_PROXY) -> BoundClient:
    """Convenience one-liner to protect your entire Python runtime."""
    client = BoundClient(proxy_url=proxy_url, agent_id=agent_id)
    client.protect()
    return client

def check(target: str, amount: float, action: str = "custom_tool", commit: bool = True) -> Tuple[bool, str, Dict[str, Any]]:
    return _default_client.check(target, amount, action, commit)

def status() -> Dict[str, Any]:
    return _default_client.status()
