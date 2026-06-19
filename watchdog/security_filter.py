"""Security filter that strips PII, API keys, passwords, and secrets
from entity/relation data before inserting into the graph.

Runs AFTER extraction but BEFORE graph CRUD calls.
Matched values are replaced with [REDACTED].
"""

from __future__ import annotations

import logging
import re
from typing import Any

logger = logging.getLogger(__name__)

REDACTED = "[REDACTED]"

API_KEY_PATTERNS: list[re.Pattern[str]] = [
    re.compile(r"sk-[A-Za-z0-9]{20,}", re.ASCII),  # OpenAI
    re.compile(r"pk-[A-Za-z0-9]{20,}", re.ASCII),  # OpenAI project key
    re.compile(r"ghp_[A-Za-z0-9]{36,}", re.ASCII),  # GitHub personal access
    re.compile(r"gho_[A-Za-z0-9]{36,}", re.ASCII),  # GitHub OAuth
    re.compile(r"ghu_[A-Za-z0-9]{36,}", re.ASCII),  # GitHub user-to-server
    re.compile(r"AKIA[0-9A-Z]{16}", re.ASCII),  # AWS access key
    re.compile(
        r"eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}", re.ASCII
    ),  # JWT
    re.compile(r"sk_live_[A-Za-z0-9]{24,}", re.ASCII),  # Stripe live
    re.compile(r"sk_test_[A-Za-z0-9]{24,}", re.ASCII),  # Stripe test
]

PASSWORD_PATTERNS: list[re.Pattern[str]] = [
    re.compile(
        r"(?:password|passwd|pwd|secret|token|apikey|api_key)\s*[:=]\s*\S+",
        re.IGNORECASE,
    ),
    re.compile(r"(?:PASS|SECRET|TOKEN)\s*[:=]\s*\S+"),
]

PRIVATE_KEY_PATTERNS: list[re.Pattern[str]] = [
    re.compile(
        r"-----BEGIN\s.*PRIVATE\sKEY-----[\s\S]*?-----END\s.*PRIVATE\sKEY-----"
    ),
    re.compile(
        r"-----BEGIN\sRSA\sPRIVATE\sKEY-----"
        r"[\s\S]*?-----END\sRSA\sPRIVATE\sKEY-----"
    ),
    re.compile(
        r"-----BEGIN\sEC\sPRIVATE\sKEY-----"
        r"[\s\S]*?-----END\sEC\sPRIVATE\sKEY-----"
    ),
    re.compile(
        r"-----BEGIN\sOPENSSH\sPRIVATE\sKEY-----"
        r"[\s\S]*?-----END\sOPENSSH\sPRIVATE\sKEY-----"
    ),
]

PII_PATTERNS: list[re.Pattern[str]] = [
    re.compile(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}"),  # email
    re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b"),  # IPv4
    re.compile(r"\b(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}\b"),  # IPv6 full
    re.compile(r"\b(?:[0-9a-fA-F]{1,4}:){1,7}:?\b"),  # IPv6 compressed
]

INTERNAL_PATH_PATTERNS: list[re.Pattern[str]] = [
    re.compile(r"\\\\[A-Za-z0-9_.$-]+\\[A-Za-z0-9_.$-]+"),  # \\server\share
    re.compile(r"//[A-Za-z0-9_.$-]+/[A-Za-z0-9_.$-]+"),  # //internal/
]

ENV_SECRET_PATTERNS: list[re.Pattern[str]] = [
    re.compile(
        r"(?:SECRET|TOKEN|PASSWORD|API_KEY|APIKEY)_?\w*\s*=\s*\S+", re.IGNORECASE
    ),
]

ALL_PATTERNS: list[re.Pattern[str]] = (
    API_KEY_PATTERNS
    + PASSWORD_PATTERNS
    + PRIVATE_KEY_PATTERNS
    + PII_PATTERNS
    + INTERNAL_PATH_PATTERNS
    + ENV_SECRET_PATTERNS
)


def _scan_value(value: str) -> str:
    """Scan a single string value. If any pattern matches, return [REDACTED]."""
    for pattern in ALL_PATTERNS:
        if pattern.search(value):
            return REDACTED
    return value


def filter_entity_data(data: dict[str, Any]) -> dict[str, Any]:
    """Return a new dict with sensitive values redacted from an entity data dict."""
    result: dict[str, Any] = {}
    for key, value in data.items():
        if isinstance(value, str):
            result[key] = _scan_value(value)
        elif isinstance(value, dict):
            result[key] = filter_entity_data(value)
        elif isinstance(value, list):
            result[key] = [
                filter_entity_data(v) if isinstance(v, dict) else v for v in value
            ]
        else:
            result[key] = value

    redacted_count = sum(1 for v in result.values() if v is REDACTED)
    if redacted_count > 0:
        logger.info(f"Redacted {redacted_count} sensitive field(s) in entity data")
    return result


def filter_relation_data(data: dict[str, Any]) -> dict[str, Any]:
    """Same as filter_entity_data but for relation data dicts."""
    return filter_entity_data(data)
