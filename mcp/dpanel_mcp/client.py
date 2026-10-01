"""Async DPanel HTTP client with JWT login and transparent token refresh."""

from __future__ import annotations

import asyncio
import time
from typing import Any

import httpx

from .config import Config


class DPanelApiError(Exception):
    """Raised when DPanel returns an error envelope or non-2xx status."""

    def __init__(self, message: str, status_code: int | None = None, payload: Any = None):
        super().__init__(message)
        self.status_code = status_code
        self.payload = payload


class DPanelClient:
    """Thin async client over the DPanel panel API (/dpanel/api).

    Auth model (verified against the Go sources):
      - POST /dpanel/api/common/user/login  {username, password, autoLogin}
        -> {code: 0, data: {token: <JWT>}}  (HS256; 24h, or 30d when autoLogin)
      - All other endpoints expect header  Authorization: Bearer <JWT>
      - All business endpoints are POST with a JSON body (even reads).
    """

    def __init__(self, config: Config):
        self.config = config
        self._http = httpx.AsyncClient(
            base_url=config.api_base,
            timeout=config.request_timeout,
            verify=config.dpanel_tls_verify,
            headers={"Content-Type": "application/json"},
        )
        self._token: str | None = None
        self._token_exp: float = 0.0
        self._lock = asyncio.Lock()

    # ------------------------------------------------------------------ auth

    async def login(self) -> str:
        """Log in and cache the JWT. Safe to call concurrently."""
        async with self._lock:
            resp = await self._http.post(
                "/common/user/login",
                json={
                    "username": self.config.dpanel_username,
                    "password": self.config.dpanel_password,
                    "autoLogin": self.config.dpanel_auto_login,
                },
            )
            data = self._unwrap(resp, expect_auth=False)
            token = (data or {}).get("token") or (data or {}).get("accessToken")
            if not token:
                raise DPanelApiError(f"login succeeded but no token in response: {data!r}")
            self._token = token
            # JWT lifetime: 24h, or 30d with autoLogin. Decode exp if possible,
            # otherwise assume the documented lifetime minus the refresh skew.
            self._token_exp = self._decode_jwt_exp(token) or (
                time.time()
                + (86400 * 30 if self.config.dpanel_auto_login else 86400)
                - self.config.token_refresh_skew
            )
            return token

    @staticmethod
    def _decode_jwt_exp(token: str) -> float | None:
        import base64
        import json

        try:
            payload_b64 = token.split(".")[1]
            payload_b64 += "=" * (-len(payload_b64) % 4)
            payload = json.loads(base64.urlsafe_b64decode(payload_b64))
            exp = payload.get("exp")
            return float(exp) if exp else None
        except Exception:
            return None

    async def _ensure_token(self) -> str:
        if self._token is None or time.time() >= self._token_exp:
            return await self.login()
        return self._token

    # -------------------------------------------------------------- requests

    @staticmethod
    def _unwrap(resp: httpx.Response, expect_auth: bool = True) -> Any:
        if resp.status_code == 401 and expect_auth:
            raise DPanelApiError("unauthorized (token expired or invalid)", 401)
        # DPanel serves the SPA shell (HTML, HTTP 200) for unknown /dpanel/api routes.
        # Detect that and give an actionable message instead of raw HTML.
        content_type = resp.headers.get("content-type", "")
        if "text/html" in content_type or resp.text.lstrip().startswith("<!DOCTYPE"):
            req_path = ""
            try:
                req_path = str(resp.request.url.path)
            except Exception:
                pass
            raise DPanelApiError(
                f"endpoint not found on DPanel: {req_path or '(unknown path)'} "
                f"(got SPA HTML). The DPanel version may not match this MCP server "
                f"(built for DPanel 1.11.x). Check DPANEL_HOST and the DPanel version.",
                resp.status_code,
            )
        try:
            body = resp.json()
        except Exception:
            raise DPanelApiError(
                f"non-JSON response (HTTP {resp.status_code}): {resp.text[:200]}",
                resp.status_code,
            )
        # DPanel envelope: {code: 200, data: ...} on success; {code: N, error: ...} otherwise.
        code = body.get("code", 0)
        if code not in (0, 200):
            raise DPanelApiError(
                body.get("error") or body.get("message") or f"dpanel error code={code}",
                resp.status_code,
                body,
            )
        return body.get("data")

    async def post(self, path: str, payload: dict | None = None, _retry: bool = True) -> Any:
        """POST to a panel API endpoint (path relative to /dpanel/api)."""
        token = await self._ensure_token()
        resp = await self._http.post(
            path,
            json=payload or {},
            headers={"Authorization": f"Bearer {token}"},
        )
        if resp.status_code == 401 and _retry:
            # token rejected early (e.g. server restarted, secret rotated) -> re-login once
            self._token = None
            token = await self.login()
            resp = await self._http.post(
                path,
                json=payload or {},
                headers={"Authorization": f"Bearer {token}"},
            )
        return self._unwrap(resp)

    async def close(self) -> None:
        await self._http.aclose()
