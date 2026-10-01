"""Redaction of sensitive values in tool output (passwords, tokens, secrets).

Applied recursively to any dict/list returned by a tool before it is
serialized to the MCP client. Redaction is always on (frozen decision 9B).
"""

from __future__ import annotations

import re
from typing import Any

MASK = "******"

# key names (lowercased, separators normalized) that always get masked
SENSITIVE_KEYS = {
    "password", "passwd", "secret", "token", "accesstoken", "refreshtoken",
    "apikey", "api_key", "accesskey", "secretkey", "privatekey", "private_key",
    "jwt", "authorization", "cookie", "session", "credential", "credentials",
    "webhooksecret", "clientsecret", "client_secret", "sshkey", "ssh_key",
    "tlskey", "tls_key", "certkey", "passphrase",
}

# environment-variable style: NAME=VALUE where NAME smells sensitive
_ENV_RE = re.compile(
    r"(?i)\b([A-Z0-9_]*(?:PASSWORD|PASSWD|SECRET|TOKEN|KEY|CREDENTIAL|JWT)[A-Z0-9_]*)\s*=\s*([^\s\"']+)"
)

# long bearer-ish strings inside free text (JWTs are xxx.yyy.zzz)
_JWT_RE = re.compile(r"\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}\b")


def _normalize_key(key: str) -> str:
    return re.sub(r"[\s_\-.\[\]]+", "", key.lower())


def _is_sensitive_key(key: str) -> bool:
    norm = _normalize_key(key)
    return any(norm == k or norm.endswith(k) for k in (_normalize_key(s) for s in SENSITIVE_KEYS))


def redact_text(text: str) -> str:
    """Mask env-style assignments and JWTs inside free-form text (e.g. logs)."""
    text = _ENV_RE.sub(lambda m: f"{m.group(1)}={MASK}", text)
    text = _JWT_RE.sub(MASK, text)
    return text


def redact(obj: Any) -> Any:
    """Recursively redact a JSON-like structure."""
    if isinstance(obj, dict):
        out = {}
        for k, v in obj.items():
            if isinstance(k, str) and _is_sensitive_key(k):
                out[k] = MASK
            else:
                out[k] = redact(v)
        return out
    if isinstance(obj, (list, tuple)):
        return [redact(v) for v in obj]
    if isinstance(obj, str):
        # only touch strings that actually look like they contain secrets,
        # to avoid mangling ordinary prose
        if _JWT_RE.search(obj) or _ENV_RE.search(obj):
            return redact_text(obj)
        return obj
    return obj
