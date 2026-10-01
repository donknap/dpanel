"""Server entrypoint: stdio or streamable-http transport."""

from __future__ import annotations

import sys

from .config import load_config
from .tools import bind, mcp


def main() -> None:
    config = load_config()
    problems = config.validate()
    if problems:
        # stdio mode: stderr only, never pollute stdout (MCP protocol channel)
        print("dpanel-mcp configuration errors:", file=sys.stderr)
        for p in problems:
            print(f"  - {p}", file=sys.stderr)
        sys.exit(2)

    bind(config)

    if config.transport == "stdio":
        mcp.run(transport="stdio")
    else:
        # streamable-http; optional bearer token gate (actually mounted on the ASGI app)
        if config.mcp_auth_token:
            from starlette.middleware.base import BaseHTTPMiddleware
            from starlette.responses import JSONResponse

            app = mcp.http_app(path=config.http_path, transport="streamable-http")

            class AuthGate(BaseHTTPMiddleware):
                async def dispatch(self, request, call_next):
                    auth = request.headers.get("Authorization", "")
                    if request.url.path == config.http_path and auth != f"Bearer {config.mcp_auth_token}":
                        return JSONResponse({"error": "unauthorized"}, status_code=401)
                    return await call_next(request)

            app.add_middleware(AuthGate)
            import uvicorn

            uvicorn.run(
                app,
                host=config.http_host,
                port=config.http_port,
                log_level="info",
            )
        else:
            mcp.run(
                transport="streamable-http",
                host=config.http_host,
                port=config.http_port,
                path=config.http_path,
            )


if __name__ == "__main__":
    main()
