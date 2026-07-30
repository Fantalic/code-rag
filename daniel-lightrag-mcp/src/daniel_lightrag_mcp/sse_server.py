import logging
import os
from contextlib import asynccontextmanager

import uvicorn
from starlette.applications import Starlette
from starlette.responses import JSONResponse
from starlette.routing import Route

from .server import _get_lightrag_url, mcp_server

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    handlers=[logging.StreamHandler()],
)
logger = logging.getLogger("sse-server")


async def health(request):
    return JSONResponse({"status": "ok", "lightrag_url": _get_lightrag_url()})


@asynccontextmanager
async def lifespan(_app):
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


base = mcp_server.sse_app()
app = Starlette(
    routes=base.routes + [Route("/health", endpoint=health)],
    lifespan=lifespan,
    middleware=base.user_middleware,
)


def main():
    host = os.getenv("MCP_HOST", "0.0.0.0")
    port = int(os.getenv("MCP_PORT", "9622"))
    logger.info(f"Starting MCP SSE server on {host}:{port}")
    logger.info(f"SSE endpoint: http://{host}:{port}/sse")
    logger.info(f"Message endpoint: http://{host}:{port}/messages/")
    uvicorn.run(app, host=host, port=port, log_level="info")


if __name__ == "__main__":
    main()
