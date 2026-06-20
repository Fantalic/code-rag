"""
SSE-based MCP server entry point for LightRAG integration.
Runs alongside LightRAG API in Docker, exposing SSE transport on port 9622.
"""

import logging
import os
from contextlib import asynccontextmanager

import uvicorn
from starlette.applications import Starlette
from starlette.routing import Mount, Route
from starlette.responses import JSONResponse

from mcp.server.sse import SseServerTransport
from mcp.server.models import InitializationOptions
from mcp.server import NotificationOptions

from .server import mcp_server, lightrag_client, _get_lightrag_url

# Set up logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    handlers=[logging.StreamHandler()],
)
logger = logging.getLogger("sse-server")

# SSE transport: messages are POSTed to /messages/
sse = SseServerTransport("/messages/")


class _ASGIApp:
    """Wraps an ASGI function so Starlette Route treats it as an ASGI app (not a request handler).

    Starlette's Route inspects endpoints: functions become request handlers (called with (request)),
    while everything else is treated as an ASGI app (called with (scope, receive, send)).
    This wrapper ensures our raw ASGI handlers are dispatched correctly.
    """

    def __init__(self, func):
        self.func = func

    async def __call__(self, scope, receive, send):
        await self.func(scope, receive, send)


async def handle_sse(scope, receive, send):
    """Handle SSE connection: client connects to /sse, receives stream."""
    async with sse.connect_sse(scope, receive, send) as streams:
        read_stream, write_stream = streams
        init_options = InitializationOptions(
            server_name="daniel-lightrag-mcp",
            server_version="0.1.0",
            capabilities=mcp_server.get_capabilities(
                notification_options=NotificationOptions(),
                experimental_capabilities={},
            ),
        )
        await mcp_server.run(read_stream, write_stream, init_options)


async def handle_messages(scope, receive, send):
    """Handle client messages POSTed to /messages/."""
    await sse.handle_post_message(scope, receive, send)


async def health(request):
    """Health check endpoint."""
    return JSONResponse({"status": "ok", "lightrag_url": _get_lightrag_url()})


@asynccontextmanager
async def lifespan(_app):
    """Startup and shutdown lifecycle."""
    global lightrag_client
    from .client import LightRAGClient

    lightrag_url = _get_lightrag_url()
    logger.info(f"Connecting to LightRAG at {lightrag_url}")
    lightrag_client = LightRAGClient(base_url=lightrag_url)
    try:
        await lightrag_client.__aenter__()
        health_data = await lightrag_client.health_check()
        logger.info(f"LightRAG health: {health_data}")
    except Exception as e:
        logger.warning(f"Could not connect to LightRAG at startup: {e}")
    yield
    if lightrag_client:
        logger.info("Closing LightRAG client...")
        await lightrag_client.__aexit__(None, None, None)


app = Starlette(
    routes=[
        Route("/sse", endpoint=_ASGIApp(handle_sse)),
        Route("/messages/", endpoint=_ASGIApp(handle_messages)),
        Route("/health", endpoint=health),
    ],
    lifespan=lifespan,
)


def main():
    """Run the SSE MCP server."""
    host = os.getenv("MCP_HOST", "0.0.0.0")
    port = int(os.getenv("MCP_PORT", "9622"))
    logger.info(f"Starting MCP SSE server on {host}:{port}")
    logger.info(f"SSE endpoint: http://{host}:{port}/sse")
    logger.info(f"Message endpoint: http://{host}:{port}/messages/")
    uvicorn.run(app, host=host, port=port, log_level="info")


if __name__ == "__main__":
    main()
