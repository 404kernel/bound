"""
BOUND Python SDK
Programmable spend vaults and financial rails for autonomous AI agents.
"""

from .client import (
    BoundClient,
    BoundError,
    protect,
    check,
    status
)

__all__ = ["BoundClient", "BoundError", "protect", "check", "status"]
__version__ = "0.2.0"
