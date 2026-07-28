"""
MCP server for LightRAG integration — 13 tools for AI agent codebase exploration.
"""

import asyncio
import hashlib
import json
import logging
import os
import time
from typing import Any, Optional

from dotenv import load_dotenv
from mcp.server import NotificationOptions, Server
from mcp.server.models import InitializationOptions
from mcp.server.stdio import stdio_server
from mcp.types import Tool

from .client import LightRAGAPIError, LightRAGClient, LightRAGError
from .models import AddNoteResponse

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)
logging.getLogger("httpx").setLevel(logging.WARNING)
logging.getLogger("mcp").setLevel(logging.INFO)


def _get_lightrag_url() -> str:
    return os.getenv(
        "LIGHTRAG_URL", os.getenv("LIGHTRAG_BASE_URL", "http://localhost:9621")
    )


mcp_server = Server("daniel-lightrag-mcp")
lightrag_client: Optional[LightRAGClient] = None


# ---------------------------------------------------------------------------
# 6 tools
# ---------------------------------------------------------------------------

TOOLS = [
    Tool(
        name="query",
        description=(
            "Query the knowledge graph using natural language. "
            "Returns AI-generated answers grounded in the codebase context. "
            "Modes: naive (keyword), local (entity), global (theme), hybrid (default)."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "Natural language query about the codebase",
                },
                "mode": {
                    "type": "string",
                    "description": "Query mode",
                    "enum": ["naive", "local", "global", "hybrid"],
                    "default": "hybrid",
                },
                "only_need_context": {
                    "type": "boolean",
                    "description": "Return only context without generated answer",
                    "default": False,
                },
            },
            "required": ["query"],
        },
    ),
    Tool(
        name="get_related_files",
        description=(
            "Find files relevant to a query by searching the knowledge graph. "
            "Returns document IDs and snippets. Useful for finding which source files "
            "relate to a concept."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "Query or keywords to find related files",
                },
                "mode": {
                    "type": "string",
                    "description": "Query mode",
                    "enum": ["naive", "local", "global", "hybrid"],
                    "default": "hybrid",
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of related files",
                    "default": 10,
                    "minimum": 1,
                    "maximum": 100,
                },
            },
            "required": ["query"],
        },
    ),
    Tool(
        name="check_entity_exists",
        description=(
            "Check if a named entity (class, function, module, concept) exists in the "
            "knowledge graph. Returns the entity ID if found."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "entity_name": {
                    "type": "string",
                    "description": (
                        "Name of the entity to check "
                        "(e.g. 'UserService', 'calculateTotal', 'main.go')"
                    ),
                }
            },
            "required": ["entity_name"],
        },
    ),
    Tool(
        name="get_knowledge_graph",
        description=(
            "Get the knowledge graph (entities + relationships). "
            "Returns nodes and edges representing indexed code elements. "
            "Use max_depth to control detail level: 1=top-level types only, "
            "2=+members, 3=all (default)."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "max_depth": {
                    "type": "integer",
                    "description": "Detail level: 1=types only, 2=+members, 3=all",
                    "default": 3,
                    "minimum": 1,
                    "maximum": 3,
                },
            },
            "required": [],
        },
    ),
    Tool(
        name="get_graph_labels",
        description=(
            "List all entity and relationship types in the knowledge graph. "
            "Helps understand what code elements have been indexed."
        ),
        inputSchema={
            "type": "object",
            "properties": {},
            "required": [],
        },
    ),
    Tool(
        name="add_note_to_entity",
        description=(
            "Attach a free-form text note to an existing knowledge graph entity. "
            "Creates a linked NOTE entity with embedded descriptions, so the note "
            "participates in semantic search. "
            "Useful for recording observations, annotations, or investigation findings."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "entity_name": {
                    "type": "string",
                    "description": "Name of the target entity to attach the note to",
                },
                "note_text": {
                    "type": "string",
                    "description": (
                        "Note text to attach (will be embedded for semantic search)"
                    ),
                },
            },
            "required": ["entity_name", "note_text"],
        },
    ),
    Tool(
        name="query_data",
        description=(
            "Query the knowledge graph and get structured results (entities, "
            "relationships, chunks, references) without LLM text generation. "
            "Use for programmatic analysis of what the graph contains about a topic."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "Query text to search for in the knowledge graph",
                },
                "mode": {
                    "type": "string",
                    "description": "Query mode",
                    "enum": ["naive", "local", "global", "hybrid", "mix"],
                    "default": "hybrid",
                },
            },
            "required": ["query"],
        },
    ),
    Tool(
        name="get_codebase_stats",
        description=(
            "Get summary statistics about the codebase: total entity and relation "
            "counts, counts broken down by type (CLASS, METHOD, etc.), and number "
            "of unique source files indexed. Useful for initial orientation."
        ),
        inputSchema={
            "type": "object",
            "properties": {},
            "required": [],
        },
    ),
    Tool(
        name="get_entity_neighbors",
        description=(
            "Get all entities directly connected to a given entity (1-hop neighbors) "
            "plus all incoming and outgoing relations. Useful for understanding "
            "what surrounds a class, method, or concept in the dependency graph."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "entity_name": {
                    "type": "string",
                    "description": "Name of the entity to find neighbors for",
                }
            },
            "required": ["entity_name"],
        },
    ),
    Tool(
        name="get_entity_details",
        description=(
            "Get full details of a specific entity including its relations. "
            "Use depth to control level: 1=entity info only, 2=+structural relations "
            "(contains/defines/has), 3=full including call graph (default)."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "entity_name": {
                    "type": "string",
                    "description": "Name of the entity to get details for",
                },
                "depth": {
                    "type": "integer",
                    "description": "Detail level: 1=entity only, 2=structural, 3=full",
                    "default": 3,
                    "minimum": 1,
                    "maximum": 3,
                },
            },
            "required": ["entity_name"],
        },
    ),
    Tool(
        name="list_entities_by_type",
        description=(
            "List all entities of a specific type with pagination. "
            "Entity types include: CLASS, METHOD, PROPERTY (from C# analyser). "
            "Returns entity names, descriptions, and file paths."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "entity_type": {
                    "type": "string",
                    "description": "Entity type to list (e.g. CLASS, METHOD, PROPERTY)",
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum entities to return",
                    "default": 100,
                    "minimum": 1,
                    "maximum": 500,
                },
                "offset": {
                    "type": "integer",
                    "description": "Offset for pagination",
                    "default": 0,
                    "minimum": 0,
                },
            },
            "required": ["entity_type"],
        },
    ),
    Tool(
        name="get_entities_in_file",
        description=(
            "Get all entities (classes, methods, properties) that come from a "
            "specific source file. Useful for answering 'what is in this file?' "
            "or 'show me all classes defined in Program.cs'."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "file_path": {
                    "type": "string",
                    "description": (
                        "File path to get entities for "
                        "(e.g. 'src/Program.cs' or 'C:/project/Program.cs')"
                    ),
                }
            },
            "required": ["file_path"],
        },
    ),
    Tool(
        name="search_entities",
        description=(
            "Search for entities by regex pattern matching on their names and "
            "descriptions. Returns all matching entities with their types and file "
            "locations. Useful for 'find all classes with 'Service' in the name'."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "pattern": {
                    "type": "string",
                    "description": "Regex pattern for entity name/description search",
                },
                "entity_type": {
                    "type": "string",
                    "description": "Optional entity type filter (e.g. CLASS, METHOD)",
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum results to return",
                    "default": 50,
                    "minimum": 1,
                    "maximum": 200,
                },
            },
            "required": ["pattern"],
        },
    ),
]


@mcp_server.list_tools()
async def handle_list_tools() -> list[Tool]:
    logger.info(f"Listing {len(TOOLS)} tools")
    return TOOLS


# ---------------------------------------------------------------------------
# Tool call handler
# ---------------------------------------------------------------------------


@mcp_server.call_tool()
async def handle_call_tool(tool_name: str, arguments: dict) -> dict:
    global lightrag_client

    if lightrag_client is None:
        base_url = _get_lightrag_url()
        api_key = os.getenv("LIGHTRAG_API_KEY", None)
        timeout = float(os.getenv("LIGHTRAG_TIMEOUT", "30.0"))
        lightrag_client = LightRAGClient(
            base_url=base_url, api_key=api_key, timeout=timeout
        )

    try:
        result = await _dispatch_tool(tool_name, arguments)
        return _success(result)
    except LightRAGError as e:
        logger.error(f"{type(e).__name__} in {tool_name}: {e}")
        return _error(tool_name, e)
    except Exception as e:
        logger.error(f"Unexpected error in {tool_name}: {e}")
        return _error(tool_name, e)


async def _dispatch_tool(tool_name: str, args: dict[str, Any]) -> Any:
    client = lightrag_client
    assert client is not None

    if tool_name == "query":
        query = args["query"]
        mode = args.get("mode", "hybrid")
        only_need_context = args.get("only_need_context", False)
        return await client.query_text(
            query, mode=mode, only_need_context=only_need_context
        )

    elif tool_name == "get_related_files":
        query = args["query"]
        mode = args.get("mode", "hybrid")
        limit = args.get("limit", 10)
        return await client.get_related_files(query, mode=mode, limit=limit)

    elif tool_name == "check_entity_exists":
        entity_name = args["entity_name"]
        return await client.check_entity_exists(entity_name)

    elif tool_name == "get_knowledge_graph":
        max_depth = args.get("max_depth", 3)
        return await client.get_knowledge_graph(max_depth=max_depth)

    elif tool_name == "get_graph_labels":
        return await client.get_graph_labels()

    elif tool_name == "add_note_to_entity":
        return await _handle_add_note(client, args["entity_name"], args["note_text"])

    elif tool_name == "query_data":
        query = args["query"]
        mode = args.get("mode", "hybrid")
        return await client.query_data(query, mode=mode)

    elif tool_name == "get_codebase_stats":
        return await client.get_codebase_stats()

    elif tool_name == "get_entity_neighbors":
        entity_name = args["entity_name"]
        return await client.get_entity_neighbors(entity_name)

    elif tool_name == "get_entity_details":
        entity_name = args["entity_name"]
        depth = args.get("depth", 3)
        return await client.get_entity_details(entity_name, depth=depth)

    elif tool_name == "list_entities_by_type":
        entity_type = args["entity_type"]
        limit = args.get("limit", 100)
        offset = args.get("offset", 0)
        return await client.list_entities_by_type(
            entity_type, limit=limit, offset=offset
        )

    elif tool_name == "get_entities_in_file":
        file_path = args["file_path"]
        return await client.get_entities_in_file(file_path)

    elif tool_name == "search_entities":
        pattern = args["pattern"]
        entity_type = args.get("entity_type")
        limit = args.get("limit", 50)
        return await client.search_entities(
            pattern, entity_type=entity_type, limit=limit
        )

    else:
        raise LightRAGAPIError(f"Unknown tool: {tool_name}")


async def _handle_add_note(
    client: LightRAGClient, entity_name: str, note_text: str
) -> AddNoteResponse:
    # 1. Check entity exists
    exists_resp = await client.check_entity_exists(entity_name)
    if not exists_resp.exists:
        return AddNoteResponse(
            status="error",
            entity_name=entity_name,
            message=f"Entity '{entity_name}' not found in the knowledge graph",
        )

    # 2. Generate a unique, deterministic note entity name
    ts_hash = hashlib.sha256(
        f"{entity_name}:{time.time()}:{note_text}".encode()
    ).hexdigest()[:12]
    note_entity_name = f"note:{entity_name}:{ts_hash}"

    # 3. Create note entity (LightRAG embeds description)
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

    # 4. Create HAS_NOTE relationship linking the target entity to the note
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


# ---------------------------------------------------------------------------
# Response helpers
# ---------------------------------------------------------------------------


def _serialize(result: Any) -> dict:
    if hasattr(result, "model_dump"):
        return result.model_dump()
    if hasattr(result, "dict"):
        return result.dict()
    if isinstance(result, (list, tuple)):
        return {"results": list(result)}
    return {"result": str(result)}


def _success(result: Any) -> dict:
    data = _serialize(result)
    return {
        "content": [{"type": "text", "text": json.dumps(data, indent=2)}],
    }


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


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


async def main() -> None:
    load_dotenv()
    logger.info(
        f"Starting LightRAG MCP server (13 tools), LightRAG URL: {_get_lightrag_url()}"
    )

    try:
        async with stdio_server() as (read_stream, write_stream):
            capabilities = mcp_server.get_capabilities(
                notification_options=NotificationOptions(),
                experimental_capabilities={},
            )
            init_options = InitializationOptions(
                server_name="daniel-lightrag-mcp",
                server_version="0.1.0",
                capabilities=capabilities,
            )
            await mcp_server.run(read_stream, write_stream, init_options)
    except KeyboardInterrupt:
        logger.info("Server shutdown requested")
    finally:
        global lightrag_client
        if lightrag_client:
            await lightrag_client.__aexit__(None, None, None)
        logger.info("LightRAG MCP server shut down")


if __name__ == "__main__":
    asyncio.run(main())
