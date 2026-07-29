"""
MCP server for LightRAG integration — 17 tools for AI agent codebase exploration.
"""

import asyncio
import hashlib
import json
import logging
import os
import time
from typing import Any

from dotenv import load_dotenv
from mcp.server import MCPServer

from .client import LightRAGAPIError, LightRAGClient, LightRAGError
from .models import AddNoteResponse

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)
logging.getLogger("httpx").setLevel(logging.WARNING)


def _get_lightrag_url() -> str:
    return os.getenv(
        "LIGHTRAG_URL", os.getenv("LIGHTRAG_BASE_URL", "http://localhost:9621")
    )


def _get_lightrag_client() -> LightRAGClient:
    base_url = _get_lightrag_url()
    api_key = os.getenv("LIGHTRAG_API_KEY", None)
    timeout = float(os.getenv("LIGHTRAG_TIMEOUT", "30.0"))
    return LightRAGClient(base_url=base_url, api_key=api_key, timeout=timeout)


mcp_server = MCPServer(
    name="daniel-lightrag-mcp",
    instructions="LightRAG knowledge graph MCP server for codebase exploration.",
)


@mcp_server.tool()
async def query(
    query: str,
    mode: str = "hybrid",
    only_need_context: bool = False,
) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.query_text(
            query, mode=mode, only_need_context=only_need_context
        )
        return _serialize(result)
    except LightRAGError as e:
        return _error("query", e)
    except Exception as e:
        return _error("query", e)


@mcp_server.tool()
async def get_related_files(
    query: str,
    mode: str = "hybrid",
    limit: int = 10,
) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.get_related_files(query, mode=mode, limit=limit)
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_related_files", e)
    except Exception as e:
        return _error("get_related_files", e)


@mcp_server.tool()
async def check_entity_exists(entity_name: str) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.check_entity_exists(entity_name)
        return _serialize(result)
    except LightRAGError as e:
        return _error("check_entity_exists", e)
    except Exception as e:
        return _error("check_entity_exists", e)


@mcp_server.tool()
async def get_knowledge_graph(max_depth: int = 3) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.get_knowledge_graph(max_depth=max_depth)
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_knowledge_graph", e)
    except Exception as e:
        return _error("get_knowledge_graph", e)


@mcp_server.tool()
async def get_graph_labels() -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.get_graph_labels()
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_graph_labels", e)
    except Exception as e:
        return _error("get_graph_labels", e)


@mcp_server.tool()
async def add_note_to_entity(entity_name: str, note_text: str) -> dict:
    client = _get_lightrag_client()
    try:
        result = await _handle_add_note(client, entity_name, note_text)
        return _serialize(result)
    except LightRAGError as e:
        return _error("add_note_to_entity", e)
    except Exception as e:
        return _error("add_note_to_entity", e)


@mcp_server.tool()
async def query_data(query: str, mode: str = "hybrid") -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.query_data(query, mode=mode)
        return _serialize(result)
    except LightRAGError as e:
        return _error("query_data", e)
    except Exception as e:
        return _error("query_data", e)


@mcp_server.tool()
async def get_codebase_stats() -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.get_codebase_stats()
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_codebase_stats", e)
    except Exception as e:
        return _error("get_codebase_stats", e)


@mcp_server.tool()
async def get_entity_neighbors(entity_name: str) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.get_entity_neighbors(entity_name)
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_entity_neighbors", e)
    except Exception as e:
        return _error("get_entity_neighbors", e)


@mcp_server.tool()
async def get_entity_details(entity_name: str, depth: int = 3) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.get_entity_details(entity_name, depth=depth)
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_entity_details", e)
    except Exception as e:
        return _error("get_entity_details", e)


@mcp_server.tool()
async def list_entities_by_type(
    entity_type: str, limit: int = 100, offset: int = 0
) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.list_entities_by_type(
            entity_type, limit=limit, offset=offset
        )
        return _serialize(result)
    except LightRAGError as e:
        return _error("list_entities_by_type", e)
    except Exception as e:
        return _error("list_entities_by_type", e)


@mcp_server.tool()
async def get_entities_in_file(file_path: str) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.get_entities_in_file(file_path)
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_entities_in_file", e)
    except Exception as e:
        return _error("get_entities_in_file", e)


@mcp_server.tool()
async def search_entities(
    pattern: str, entity_type: str = None, limit: int = 50
) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.search_entities(
            pattern, entity_type=entity_type, limit=limit
        )
        return _serialize(result)
    except LightRAGError as e:
        return _error("search_entities", e)
    except Exception as e:
        return _error("search_entities", e)


async def _handle_add_note(
    client: LightRAGClient, entity_name: str, note_text: str
) -> AddNoteResponse:
    exists_resp = await client.check_entity_exists(entity_name)
    if not exists_resp.exists:
        return AddNoteResponse(
            status="error",
            entity_name=entity_name,
            message=f"Entity '{entity_name}' not found in the knowledge graph",
        )

    ts_hash = hashlib.sha256(
        f"{entity_name}:{time.time()}:{note_text}".encode()
    ).hexdigest()[:12]
    note_entity_name = f"note:{entity_name}:{ts_hash}"

    entity_data = {
        "description": note_text,
        "entity_type": "NOTE",
    }
    create_resp = await client.create_entity(note_entity_name, entity_data)
    if not create_resp.created and "success" not in create_resp.message.lower():
        return AddNoteResponse(
            status="error",
            entity_name=entity_name,
            message=f"Failed to create note entity: {create_resp.message}",
        )

    relation_data = {
        "description": f"Note attached to {entity_name}",
        "keywords": ["note", "annotation"],
        "weight": 1.0,
    }
    relation_resp = await client.create_relation(
        entity_name, note_entity_name, relation_data
    )

    return AddNoteResponse(
        status="success",
        entity_name=entity_name,
        note_entity_name=note_entity_name,
        note_text=note_text,
        relationship_created=relation_resp.created,
        message="Note entity created and linked via HAS_NOTE relationship",
    )


@mcp_server.tool()
async def get_relation_detail(
    entity_name: str, relation_type: str
) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.get_relation_detail(
            entity_name, relation_type
        )
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_relation_detail", e)
    except Exception as e:
        return _error("get_relation_detail", e)


@mcp_server.tool()
async def get_method_signature(entity_name: str) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.get_method_signature(entity_name)
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_method_signature", e)
    except Exception as e:
        return _error("get_method_signature", e)


@mcp_server.tool()
async def find_related_files(entity_or_file: str) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.find_related_files(entity_or_file)
        return _serialize(result)
    except LightRAGError as e:
        return _error("find_related_files", e)
    except Exception as e:
        return _error("find_related_files", e)


@mcp_server.tool()
async def get_interface_methods(interface_name: str) -> dict:
    client = _get_lightrag_client()
    try:
        result = await client.get_interface_methods(interface_name)
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_interface_methods", e)
    except Exception as e:
        return _error("get_interface_methods", e)


def _serialize(result: Any) -> dict:
    if hasattr(result, "model_dump"):
        return result.model_dump()
    if hasattr(result, "dict"):
        return result.dict()
    if isinstance(result, (list, tuple)):
        return {"results": list(result)}
    return {"result": str(result)}


def _error(tool_name: str, error: Exception) -> dict:
    details = {
        "tool": tool_name,
        "error_type": type(error).__name__,
        "message": str(error),
    }
    if isinstance(error, LightRAGError):
        details["status_code"] = str(error.status_code) if error.status_code else "N/A"
    return {
        "content": [{"type": "text", "text": json.dumps(details, indent=2)}],
        "isError": True,
    }


async def main() -> None:
    load_dotenv()
    logger.info(
        f"Starting LightRAG MCP server (17 tools), LightRAG URL: {_get_lightrag_url()}"
    )

    try:
        await mcp_server.run_stdio_async()
    except KeyboardInterrupt:
        logger.info("Server shutdown requested")
    finally:
        logger.info("LightRAG MCP server shut down")


if __name__ == "__main__":
    asyncio.run(main())