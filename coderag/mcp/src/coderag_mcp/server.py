"""
MCP server for LightRAG integration — curated tools for AI agent codebase exploration.

The `query` tool (LLM-backed natural-language answer) is kept as a Python function
but is NOT registered with FastMCP — it is reserved for the `coderag` CLI.
All other tools are deterministic (graph or structured /query/data) and stay
exposed to AI agents. Tool count is logged at startup from `mcp_server.list_tools()`.
"""

import asyncio
import hashlib
import json
import logging
import os
import time
from typing import Any

from dotenv import load_dotenv

from mcp.server.fastmcp import FastMCP

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


mcp_server = FastMCP(
    name="coderag-mcp",
    instructions="LightRAG knowledge graph MCP server for codebase exploration.",
)


# @mcp_server.tool()  # CLI-only: not registered with FastMCP.
async def query(
    query: str,
    mode: str = "hybrid",
    only_need_context: bool = False,
) -> dict:
    raise NotImplementedError(
        "The 'query' natural-language tool is not exposed via MCP. "
        "Use the `coderag` CLI directly."
    )


# @mcp_server.tool()  # Dropped from MCP: redundant with query_codebase.
async def get_related_files(
    query: str,
    mode: str = "hybrid",
    limit: int = 10,
) -> dict:
    raise NotImplementedError(
        "The 'get_related_files' tool was dropped from MCP. "
        "Use query_codebase and read its 'references' field, or a direct file tool."
    )


@mcp_server.tool(name="file_dependencies")
async def get_file_dependencies(file_path: str) -> dict:
    """Files that depend on or are depended on by a given file, e.g. "src/Services/OrderService.cs". See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        result = await client.get_file_dependencies(file_path)
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_file_dependencies", e)
    except Exception as e:
        return _error("get_file_dependencies", e)


@mcp_server.tool(name="detect_circular_dependencies")
async def file_dependency_cycles() -> dict:
    """Detect circular dependencies in the codebase. See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        result = await client.file_dependency_cycles()
        return _serialize(result)
    except LightRAGError as e:
        return _error("file_dependency_cycles", e)
    except Exception as e:
        return _error("file_dependency_cycles", e)


# @mcp_server.tool()
async def get_file_stats() -> dict:
    """Not exposed as MCP tool. A dedicated `dead_code` tool is planned; see TASK_dead_code_identifier.md."""
    raise NotImplementedError(
        "get_file_stats is not exposed. A dead_code tool is planned for the future."
    )


@mcp_server.tool(name="exists_in_codebase")
async def check_entity_exists(entity_name: str) -> dict:
    """True if a class, method, interface, property, namespace, or enum is in the codebase by qualified name (e.g. "MyApp.OrderService.PlaceOrder"). See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        result = await client.check_entity_exists(entity_name)
        return _serialize(result)
    except LightRAGError as e:
        return _error("check_entity_exists", e)
    except Exception as e:
        return _error("check_entity_exists", e)


@mcp_server.tool(name="codebase_overview")
async def get_knowledge_graph(max_depth: int = 1) -> dict:
    """An overview of the codebase as a graph (nodes + edges); pass max_depth to go deeper than the default top-level view. See `coderag --agent-help`."""
    # Default depth=1: top-level nodes only, fast orientation pass.
    # Future: response may grow to include higher-level summaries (project
    # structure, type counts, hotspots) so the agent doesn't have to walk the
    # full graph just to orient. Will tune the default in action once we know
    # what agents actually need.
    client = _get_lightrag_client()
    try:
        result = await client.get_knowledge_graph(max_depth=max_depth)
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_knowledge_graph", e)
    except Exception as e:
        return _error("get_knowledge_graph", e)


# @mcp_server.tool()
async def get_graph_labels(
    node: str = "",
    limit: int = 100,
    offset: int = 0,
) -> dict:
    """Not exposed as MCP tool. Entity/relation type info is available via `get_nodes` and `node_details(view='neighbors'|'relations')`."""
    raise NotImplementedError(
        "codebase_types is not exposed. Use get_nodes(entity_type='class') for a list of "
        "entities of a given type, or node_details(view='neighbors'|'relations') "
        "for type information about a specific node's connections."
    )


@mcp_server.tool(name="attach_note_to_node")
async def add_note_to_entity(entity_name: str, note_text: str) -> dict:
    """Attach a free-text note to a node to record guidance, observations, or context for future agents. See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        result = await _handle_add_note(client, entity_name, note_text)
        return _serialize(result)
    except LightRAGError as e:
        return _error("add_note_to_entity", e)
    except Exception as e:
        return _error("add_note_to_entity", e)


@mcp_server.tool(name="get_notes")
async def get_notes_for_node(node_name: str) -> dict:
    """Return all notes attached to a node (most recent first). See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        notes = await _list_notes_for_node(client, node_name)
        return {"node": node_name, "notes": notes, "total": len(notes)}
    except LightRAGError as e:
        return _error("get_notes", e)
    except Exception as e:
        return _error("get_notes", e)


@mcp_server.tool(name="query_codebase")
async def query_data(query: str, mode: str = "hybrid") -> dict:
    """Query the codebase and return matching entities, relations, chunks, and file references. See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        result = await client.query_data(query, mode=mode)
        return _serialize(result)
    except LightRAGError as e:
        return _error("query_data", e)
    except Exception as e:
        return _error("query_data", e)


# @mcp_server.tool()
async def get_entity_neighbors(entity_name: str) -> dict:
    """Not exposed as MCP tool. 1-hop neighbors are available via `node_details(view="neighbors")`."""
    raise NotImplementedError(
        "node_neighbors is not exposed. Use node_details(entity_name, view='neighbors')."
    )


@mcp_server.tool(name="node_details")
async def get_node_details(
    entity_name: str,
    view: str = "details",
    relation_type: str = "",
    depth: int = 2,
) -> dict:
    """Unified node lookup. `view` selects the response shape: "details" (default; node + incoming + outgoing + 1-hop neighbors), "relations" (just edges, optionally filtered by `relation_type`), or "neighbors" (1-hop neighbor entities + connecting relations). See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        result = await client.get_node_details(
            entity_name=entity_name,
            view=view,
            relation_type=relation_type,
            depth=depth,
        )
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_node_details", e)
    except Exception as e:
        return _error("get_node_details", e)


# @mcp_server.tool()
async def list_entities_by_type(
    entity_type: str,
    node: str = "",
    limit: int = 100,
    offset: int = 0,
) -> dict:
    """Not exposed as MCP tool. Superseded by `get_nodes` (filter by anything)."""
    raise NotImplementedError(
        "nodes_by_type is not exposed. Use get_nodes(entity_type='class', "
        "connected_to='Foo.Bar') instead — get_nodes combines type/name/file/connection filters."
    )


@mcp_server.tool(name="get_nodes")
async def get_nodes(
    entity_type: str = "",
    name: str = "",
    file_path: str = "",
    connected_to: str = "",
    limit: int = 100,
    offset: int = 0,
) -> dict:
    """Filter nodes by any combination of: `entity_type` (e.g. "class"), `name` (substring), `file_path` (substring), `connected_to` (only nodes touching this one). Empty/omitted filters are no-ops; all filters AND-combine. Paginated via `limit`/`offset`. See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        result = await client.get_nodes(
            entity_type=entity_type,
            name=name,
            file_path=file_path,
            connected_to=connected_to,
            limit=limit,
            offset=offset,
        )
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_nodes", e)
    except Exception as e:
        return _error("get_nodes", e)


@mcp_server.tool(name="nodes_in_file")
async def get_entities_in_file(file_path: str) -> dict:
    """Return all nodes defined in a file. See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        result = await client.get_entities_in_file(file_path)
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_entities_in_file", e)
    except Exception as e:
        return _error("get_entities_in_file", e)


# @mcp_server.tool()
async def search_entities(
    pattern: str, entity_type: str = None, limit: int = 50
) -> dict:
    """Not exposed as MCP tool yet — usefulness is TBD (use query_codebase or get_nodes instead)."""
    raise NotImplementedError(
        "search_entities is not exposed as an MCP tool. "
        "Use query_codebase or nodes_by_type instead."
    )


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

    ts_ms = int(time.time() * 1000)
    ts_hash = hashlib.sha256(f"{entity_name}:{ts_ms}:{note_text}".encode()).hexdigest()[
        :6
    ]
    iso_ts = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(ts_ms / 1000.0))
    note_entity_name = f"note:{ts_ms}:{ts_hash}"

    entity_data = {
        "description": f"[agent annotation @ {iso_ts}] {note_text}",
        "entity_type": "NOTE",
        "keywords": ["note", "annotation", "agent"],
        "source_id": "coderag-mcp",
    }
    create_resp = await client.create_entity(note_entity_name, entity_data)
    if create_resp.status != "success" and "success" not in create_resp.message.lower():
        return AddNoteResponse(
            status="error",
            entity_name=entity_name,
            message=f"Failed to create note entity: {create_resp.message}",
        )

    relation_data = {
        "description": f"Annotation attached at {iso_ts}",
        "keywords": "has_note,annotation",
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


async def _list_notes_for_node(
    client: LightRAGClient, node_name: str
) -> list[dict[str, Any]]:
    """Walk the graph for HAS_NOTE edges pointing to `node_name`.

    Returns NOTE entities sorted newest-first by their sortable name prefix
    (note:{timestamp_ms}:...).
    """
    graph = await client.get_knowledge_graph(label="*")
    note_names: set[str] = set()
    for edge in graph.edges:
        src = edge.get("source") or edge.get("src_id") or ""
        tgt = edge.get("target") or edge.get("tgt_id") or ""
        kw = str(edge.get("properties", {}).get("keywords", "")).lower()
        if tgt == node_name and "has_note" in kw:
            note_names.add(src)

    notes: list[dict[str, Any]] = []
    by_name: dict[str, dict[str, Any]] = {}
    for n in graph.nodes:
        props = n.get("properties") or {}
        name = props.get("entity_name") or n.get("id") or n.get("name") or ""
        if name in note_names:
            by_name[name] = {
                "note_entity_name": name,
                "description": props.get("description", ""),
                "created_at": _extract_iso_timestamp(
                    name, props.get("description", "")
                ),
            }
    for name in sorted(note_names, reverse=True):
        if name in by_name:
            notes.append(by_name[name])
    return notes


def _extract_iso_timestamp(note_name: str, description: str) -> str:
    """Best-effort ISO timestamp from note name (preferred) or description."""
    try:
        if note_name.startswith("note:"):
            parts = note_name.split(":")
            if len(parts) >= 2:
                ts_ms = int(parts[1])
                return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(ts_ms / 1000.0))
    except (ValueError, IndexError):
        pass
    if "[agent annotation @ " in description:
        try:
            return description.split("[agent annotation @ ", 1)[1].split("]", 1)[0]
        except IndexError:
            pass
    return ""


# @mcp_server.tool()
async def get_relation_detail(entity_name: str, relation_type: str = "") -> dict:
    """Not exposed as MCP tool. Relations are available via `node_details(view="relations")`."""
    raise NotImplementedError(
        "node_relations is not exposed. Use node_details(entity_name, view='relations', "
        "relation_type='...') for relations involving a node."
    )


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


# @mcp_server.tool()
async def get_interface_methods(interface_name: str) -> dict:
    """Not exposed as MCP tool. Interface method info is available via `node_details(view='details'|'neighbors')`."""
    raise NotImplementedError(
        "get_interface_methods is not exposed. Use node_details(view='details'|'neighbors') "
        "to inspect interface members and implementing classes."
    )


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
    tool_names = sorted(t.name for t in await mcp_server.list_tools())
    logger.info(
        f"Starting LightRAG MCP server ({len(tool_names)} tools), "
        f"LightRAG URL: {_get_lightrag_url()}"
    )
    logger.debug(f"Registered tools: {tool_names}")

    try:
        await mcp_server.run_stdio_async()
    except KeyboardInterrupt:
        logger.info("Server shutdown requested")
    finally:
        logger.info("LightRAG MCP server shut down")


if __name__ == "__main__":
    asyncio.run(main())
