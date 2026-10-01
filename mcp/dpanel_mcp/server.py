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
        # streamable-http; optional bearer token gate
        if config.mcp_auth_token:
            from starlette.requests import Request
            from starlette.responses import JSONResponse

            async def auth_middleware(request: Request, call_next):
                auth = request.headers.get("Authorization", "")
                if request.url.path == config.http_path and auth != f"Bearer {config.mcp_auth_token}":
                    return JSONResponse({"error": "unauthorized"}, status_code=401)
                return await call_next(request)

            mcp._mcp_server  # noqa: B018 - keep reference alive
            # FastMCP exposes the underlying ASGI app
            app = mcp.http_app(path=config.http_path, transport="streamable-http")
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
