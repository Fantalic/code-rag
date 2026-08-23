"""
MCP server for LightRAG integration — curated tools for AI agent codebase exploration.

The `query` tool (LLM-backed natural-language answer) is kept as a Python function
but is NOT registered with FastMCP — it is reserved for the `coderag` CLI.
All other tools are deterministic (graph walks or LLM-free embedding-based
/query/data search) and stay exposed to AI agents. Tool count is logged at
startup from `mcp_server.list_tools()`.
"""

import asyncio
import hashlib
import json
import logging
import os
import time
from typing import Any, Optional

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
async def get_knowledge_graph(
    max_depth: int = 1,
    view: str = "summary",
    limit: int = 200,
    offset: int = 0,
) -> dict:
    """Overview of the codebase graph. `view="summary"` (default) returns counts only (nodes/edges by type, files indexed) — start here to size up the codebase. `view="graph"` returns one page of nodes+edges (`limit`/`offset`, hard cap 1000). `max_depth` 1=top-level types, 2=+members, 3=all. See `coderag --agent-help`."""
    # Output guard: the full unpaginated dump exceeded 227 KB on a mid-size
    # monorepo and burned agent context; summary-first + pagination keeps
    # responses proportional to what was asked.
    view_norm = (view or "summary").lower()
    if view_norm not in ("summary", "graph"):
        return _error(
            "get_knowledge_graph",
            ValueError(f"Invalid view '{view}'. Must be 'summary' or 'graph'"),
        )
    limit = max(1, min(limit, 1000))
    offset = max(0, offset)
    depth = max(1, min(max_depth, 3))

    client = _get_lightrag_client()
    try:
        graph = await client.get_knowledge_graph(max_depth=depth)
        if view_norm == "summary":
            return _serialize(client.summarize_graph(graph, max_depth=depth))
        return _serialize(
            client.paginate_graph(graph, max_depth=depth, offset=offset, limit=limit)
        )
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
async def query_data(query: str) -> dict:
    """Semantic vector search over the codebase — no LLM involved: the query is embedded with the configured embedding model and matched against entity/relationship vector stores. Returns matching entities (with source spans when known) and relationships. Note: text-chunk hits only exist for prose documents ingested via LLM ingestion — indexed source files live in the graph, not the chunk store; use get_source(entity_name) for exact code lines. See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        result = await client.query_data(query)
        return _serialize(result)
    except LightRAGError as e:
        return _error("query_data", e)
    except Exception as e:
        return _error("query_data", e)


@mcp_server.tool(name="index_coverage")
async def index_coverage() -> dict:
    """Disclose what the indexer did and did NOT index: per-project file counts, skipped files with reasons, and scan timestamps — so silent coverage gaps become visible. Requires a scan with COVERAGE reporting (coderag >= current). See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        graph = await client.get_knowledge_graph(label="*")
    except LightRAGError as e:
        return _error("index_coverage", e)
    except Exception as e:
        return _error("index_coverage", e)

    projects: list[dict[str, Any]] = []
    for node in graph.nodes:
        props = node.get("properties") or {}
        if (props.get("entity_type") or "").upper() != "COVERAGE":
            continue
        raw = props.get("description") or ""
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError:
            payload = {"project": _node_name(node), "parse_error": raw[:200]}
        projects.append(payload)
    projects.sort(key=lambda p: str(p.get("project", "")))
    total_skipped = sum(len(p.get("skipped") or []) for p in projects)
    return {
        "projects": projects,
        "project_count": len(projects),
        "total_files_skipped": total_skipped,
        "hint": (
            "No COVERAGE records found means the last scan predates coverage "
            "reporting — re-run 'coderag scan' to populate."
            if not projects
            else "Files listed under 'skipped' are NOT in the graph."
        ),
    }


@mcp_server.tool(name="what_changed")
async def what_changed(hours: float = 24.0, since: str = "") -> dict:
    """List recent index changes (added/removed/updated symbols per file) from CHANGELOG records written by watch-mode re-scans. `hours` windows back from now; `since` (ISO like 2026-08-23T10:00:00Z or epoch seconds) overrides `hours` when given. See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        graph = await client.get_knowledge_graph(label="*")
    except LightRAGError as e:
        return _error("what_changed", e)
    except Exception as e:
        return _error("what_changed", e)

    cutoff = _resolve_cutoff(hours=hours, since=since)
    changes: list[dict[str, Any]] = []
    for node in graph.nodes:
        props = node.get("properties") or {}
        if (props.get("entity_type") or "").upper() != "CHANGELOG":
            continue
        entry_ts = _entry_timestamp(props.get("description") or "")
        if cutoff is not None and entry_ts is not None and entry_ts < cutoff:
            continue
        fp = node.get("file_path") or props.get("file_path", "")
        try:
            payload = json.loads(props.get("description") or "{}")
        except json.JSONDecodeError:
            payload = {"parse_error": (props.get("description") or "")[:200]}
        changes.append(
            {"file_path": fp, "changed_at": payload.get("changed_at"), **payload}
        )
    changes.sort(key=lambda c: str(c.get("changed_at") or ""), reverse=True)
    return {
        "changes": changes,
        "total": len(changes),
        "cutoff": since if since else (f"now-{hours}h" if cutoff else None),
        "hint": (
            "No CHANGELOG records yet — they are written by watch-mode "
            "(coderag mcp) re-scans after this feature shipped."
            if not changes
            else ""
        ),
    }


@mcp_server.tool(name="get_source")
async def get_source(
    entity_name: str = "",
    file_path: str = "",
    start_line: int = 0,
    end_line: int = 0,
    max_lines: int = 120,
) -> dict:
    """Read exact source lines for an entity using its indexed span — turns a graph hit into a surgical read without leaving coderag. Pass `entity_name` alone (span + file resolved from the graph), or `file_path` + optional explicit `start_line`/`end_line`. Requires SOURCE_ROOT to point at the analyzed workspace (bind-mount it into the container). See `coderag --agent-help`."""
    root = os.getenv("SOURCE_ROOT", "").strip()
    if not root:
        return _error(
            "get_source",
            RuntimeError(
                "SOURCE_ROOT is not configured. Bind-mount the analyzed repo "
                "read-only into the MCP container and set SOURCE_ROOT to that "
                "path (see docker-compose.yml example)."
            ),
        )
    client = _get_lightrag_client()
    try:
        if entity_name and not (start_line and end_line):
            graph = await client.get_knowledge_graph(label="*")
            node = client.find_entity(graph, entity_name)
            if node is None:
                return _error(
                    "get_source", LookupError(f"Entity '{entity_name}' not found")
                )
            props = node.get("properties") or {}
            file_path = node.get("file_path") or props.get("file_path") or file_path
            start_line = int(props.get("start_line") or 0)
            end_line = int(props.get("end_line") or 0)
    except LightRAGError as e:
        return _error("get_source", e)
    except Exception as e:
        return _error("get_source", e)

    if not file_path:
        return _error("get_source", ValueError("No file_path available"))

    try:
        snippet = _read_source_lines(root, file_path, start_line, end_line, max_lines)
    except (OSError, ValueError) as e:
        return _error("get_source", e)
    return {
        "file_path": file_path,
        "start_line": snippet["start"],
        "end_line": snippet["end"],
        "total_lines_in_file": snippet["total"],
        "truncated": snippet["truncated"],
        "content": snippet["content"],
    }


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
    limit: int = 50,
) -> dict:
    """Unified node lookup with per-view response shapes: "details" (default; entity + incoming/outgoing relations, depth-filtered via structural keywords), "relations" (only edges touching the node, optional `relation_type` filter), "neighbors" (1-hop neighbor entities + connecting edges, capped by `limit`, totals + `truncated` flag included). Names resolve exactly first, then unique qualified suffix (e.g. "Snake.Tile" finds "Ns.Snake.Tile"). See `coderag --agent-help`."""
    # Output guard: hub nodes (base classes referenced everywhere) used to
    # return unbounded neighborhoods; `limit` caps each direction and the
    # totals still reflect true counts.
    limit = max(1, min(limit, 200))
    client = _get_lightrag_client()
    try:
        result = await client.get_node_details(
            entity_name=entity_name,
            view=view,
            relation_type=relation_type,
            depth=depth,
            limit=limit,
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
    file_match: str = "contains",
    limit: int = 100,
    offset: int = 0,
) -> dict:
    """Filter nodes by any combination of: `entity_type` (e.g. "class"), `name` (substring), `file_path` (+ `file_match`: "contains" default or "exact" — use exact to avoid Snake matching Snake1/Snake2), `connected_to` (only nodes touching this one). Empty/omitted filters are no-ops; all filters AND-combine. Paginated via `limit`/`offset`. Entities include `start_line`/`end_line` source spans when known. See `coderag --agent-help`."""
    client = _get_lightrag_client()
    try:
        result = await client.get_nodes(
            entity_type=entity_type,
            name=name,
            file_path=file_path,
            connected_to=connected_to,
            file_match=file_match,
            limit=limit,
            offset=offset,
        )
        return _serialize(result)
    except LightRAGError as e:
        return _error("get_nodes", e)
    except Exception as e:
        return _error("get_nodes", e)


@mcp_server.tool(name="dead_code")
async def find_dead_code(
    min_age_days: int = 0,
    entity_type: str = "",
) -> dict:
    """Find symbols (classes, methods, properties) that are unreferenced and not exported from their file; useful for cleanup suggestions. `min_age_days` filters by file modification time (0 = all). `entity_type` restricts to one of "CLASS"/"METHOD"/"PROPERTY" (case-insensitive). See TASK_dead_code_implementation.md."""
    client = _get_lightrag_client()
    try:
        result = await client.find_dead_code(
            min_age_days=min_age_days,
            entity_type=entity_type,
        )
        return _serialize(result)
    except LightRAGError as e:
        return _error("find_dead_code", e)
    except Exception as e:
        return _error("find_dead_code", e)


# @mcp_server.tool()
async def get_entities_in_file(file_path: str) -> dict:
    """Not exposed as MCP tool. Superseded by `get_nodes(file_path=...)` which uses substring matching (more robust on Windows paths)."""
    raise NotImplementedError(
        "nodes_in_file is not exposed. Use get_nodes(file_path='OrderService.cs') instead."
    )


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
        if "has_note" not in kw:
            continue
        # attach_note_to_node creates  annotated --HAS_NOTE--> note ;
        # accept the reverse direction too so legacy/other writers still list.
        if tgt == node_name:
            note_names.add(src)
        elif src == node_name:
            note_names.add(tgt)

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


# @mcp_server.tool()
async def find_related_files(entity_or_file: str) -> dict:
    """Not exposed as MCP tool. Use `get_nodes(connected_to=...)` for entities touching X, then group by `file_path` to derive the related-files view."""
    raise NotImplementedError(
        "find_related_files is not exposed. Use get_nodes(connected_to='X') and group by "
        "file_path to find files related to X."
    )


# @mcp_server.tool()
async def get_interface_methods(interface_name: str) -> dict:
    """Not exposed as MCP tool. Interface method info is available via `node_details(view='details'|'neighbors')`."""
    raise NotImplementedError(
        "get_interface_methods is not exposed. Use node_details(view='details'|'neighbors') "
        "to inspect interface members and implementing classes."
    )


def _node_name(node: dict[str, Any]) -> str:
    """Display name of a graph node (entity_name > name > id)."""
    props = node.get("properties") or {}
    return props.get("entity_name") or node.get("name") or node.get("id") or ""


def _resolve_cutoff(hours: float, since: str) -> Optional[float]:
    """Epoch-seconds cutoff from an explicit `since` or an `hours` window."""
    since = (since or "").strip()
    if since:
        # Epoch seconds first, then ISO-8601.
        try:
            return float(since)
        except ValueError:
            pass
        try:
            from datetime import datetime, timezone

            iso = since.replace("Z", "+00:00")
            dt = datetime.fromisoformat(iso)
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=timezone.utc)
            return dt.timestamp()
        except ValueError:
            raise ValueError(
                f"Invalid since '{since}'. Use epoch seconds or ISO-8601 "
                "(e.g. 2026-08-23T10:00:00Z)"
            )
    if hours and hours > 0:
        return time.time() - hours * 3600.0
    return None


def _entry_timestamp(description: str) -> Optional[float]:
    """Extract 'changed_at' epoch seconds from a CHANGELOG JSON description."""
    try:
        payload = json.loads(description)
    except json.JSONDecodeError:
        return None
    raw = payload.get("changed_at")
    if not raw:
        return None
    try:
        return float(raw)
    except (TypeError, ValueError):
        pass
    try:
        from datetime import datetime, timezone

        dt = datetime.fromisoformat(str(raw).replace("Z", "+00:00"))
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt.timestamp()
    except ValueError:
        return None


def _read_source_lines(
    root: str,
    file_path: str,
    start_line: int,
    end_line: int,
    max_lines: int,
) -> dict[str, Any]:
    """Read a line range from a workspace file, guarded against path escape.

    Line numbers are 1-based inclusive; when no span is given the head of the
    file (up to max_lines) is returned so callers always get bounded output.
    """
    from pathlib import Path, PurePosixPath, PureWindowsPath

    root_path = Path(root).resolve()
    candidate = (
        PureWindowsPath(file_path) if "\\" in file_path else PurePosixPath(file_path)
    )
    full = (root_path / str(candidate)).resolve()
    if root_path != full and root_path not in full.parents:
        raise ValueError(f"Refusing to read outside SOURCE_ROOT: {file_path}")
    if not full.is_file():
        raise FileNotFoundError(f"'{file_path}' not found under SOURCE_ROOT='{root}'")

    text = full.read_text(encoding="utf-8", errors="replace")
    lines = text.splitlines()
    total = len(lines)

    lo = max(1, int(start_line or 1))
    hi = min(total, int(end_line) if end_line else total)
    hi = min(hi, lo + max_lines - 1)
    truncated = (
        (int(end_line or total)) > hi
        or total > max_lines
        and (int(start_line or 1) == 1 and int(end_line or 0) == 0)
    )
    content = "\n".join(lines[lo - 1 : hi])
    return {
        "start": lo,
        "end": hi,
        "total": total,
        "truncated": truncated or hi < min(total, int(end_line or total)),
        "content": content,
    }


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
