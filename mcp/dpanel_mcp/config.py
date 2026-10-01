"""DPanel MCP Server configuration (env-driven)."""

from __future__ import annotations

import os
from dataclasses import dataclass, field


@dataclass
class Config:
    """Runtime configuration, all overridable via environment variables."""

    # --- DPanel connection ---
    # e.g. http://127.0.0.1:8807  (no trailing slash; /dpanel/api is appended)
    dpanel_host: str = field(
        default_factory=lambda: os.getenv("DPANEL_HOST", "http://127.0.0.1:8807").rstrip("/")
    )
    dpanel_username: str = field(default_factory=lambda: os.getenv("DPANEL_USERNAME", ""))
    dpanel_password: str = field(default_factory=lambda: os.getenv("DPANEL_PASSWORD", ""))
    # autoLogin -> 30d token; otherwise 24h token
    dpanel_auto_login: bool = field(
        default_factory=lambda: os.getenv("DPANEL_AUTO_LOGIN", "1") not in ("0", "false", "no")
    )
    # verify TLS for the DPanel instance
    dpanel_tls_verify: bool = field(
        default_factory=lambda: os.getenv("DPANEL_TLS_VERIFY", "1") not in ("0", "false", "no")
    )

    # --- MCP capability profile ---
    # read-only | read-write | admin
    profile: str = field(
        default_factory=lambda: os.getenv("DPANEL_MCP_PROFILE", "read-write").lower()
    )

    # --- transport ---
    # stdio | streamable-http
    transport: str = field(
        default_factory=lambda: os.getenv("DPANEL_MCP_TRANSPORT", "stdio").lower()
    )
    http_host: str = field(default_factory=lambda: os.getenv("DPANEL_MCP_HTTP_HOST", "127.0.0.1"))
    http_port: int = field(
        default_factory=lambda: int(os.getenv("DPANEL_MCP_HTTP_PORT", "8090"))
    )
    http_path: str = field(default_factory=lambda: os.getenv("DPANEL_MCP_HTTP_PATH", "/mcp"))
    # optional bearer token gating the streamable-http endpoint itself
    mcp_auth_token: str = field(default_factory=lambda: os.getenv("DPANEL_MCP_AUTH_TOKEN", ""))

    # --- http client ---
    request_timeout: float = field(
        default_factory=lambda: float(os.getenv("DPANEL_MCP_REQUEST_TIMEOUT", "30"))
    )
    # pro-actively refresh the JWT this many seconds before it expires
    token_refresh_skew: float = field(
        default_factory=lambda: float(os.getenv("DPANEL_MCP_TOKEN_REFRESH_SKEW", "300"))
    )

    def __post_init__(self) -> None:
        # normalize: strip trailing slashes so api_base never doubles them
        self.dpanel_host = self.dpanel_host.rstrip("/")

    @property
    def api_base(self) -> str:
        return f"{self.dpanel_host}/dpanel/api"

    def validate(self) -> list[str]:
        problems = []
        if not self.dpanel_username:
            problems.append("DPANEL_USERNAME is required")
        if not self.dpanel_password:
            problems.append("DPANEL_PASSWORD is required")
        if self.profile not in ("read-only", "read-write", "admin"):
            problems.append(f"invalid DPANEL_MCP_PROFILE: {self.profile}")
        if self.transport not in ("stdio", "streamable-http"):
            problems.append(f"invalid DPANEL_MCP_TRANSPORT: {self.transport}")
        return problems


def load_config() -> Config:
    return Config()
