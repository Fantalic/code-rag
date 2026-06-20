# Phase 21 — coderag Restructuring ✅ COMPLETE

## Goal

Restructure the monorepo so `coderag` is the central CLI tool. File watcher + analyser pipeline stays as a host-side Go binary. The Python MCP server (`daniel-lightrag-mcp`) is **kept** and runs inside Docker alongside LightRAG, exposing SSE transport on port 9622.

## What Was Done

### Done ✅
- Root `go.mod` with module `github.com/code-lightrag/coderag`
- `watchdog/internal/*` moved to `pkg/watchdog/*`
- `pkg/lightrag/` HTTP client created
- `cmd/coderag/main.go` CLI with `start`, `watch`, `status`, `stop`, `query`, `webui`
- Python MCP SSE server at `daniel-lightrag-mcp/src/daniel_lightrag_mcp/sse_server.py`
- Updated `pyproject.toml` with `starlette` + `uvicorn` deps
- Updated `docker-compose.yml`: removed Ollama, added `mcp` service on :9622
- Updated `Dockerfile`: multi-stage (WebUI + LightRAG + MCP)
- Created `.opencode/opencode.json`: remote MCP at `http://localhost:9622/sse`
- Removed stale `pkg/mcp/` directory
- Updated all docs: `DOCUMENTATION.md`, `PLAN.md`, `CURRENT_CONTEXT.md`, `AGENTS.md`
- **Built and verified both Docker images** — `lightrag` and `mcp` both compile and start
- **Fixed Starlette ASGI detection bug** — `_ASGIApp` wrapper class for raw ASGI handlers in `sse_server.py`
- **Verified MCP SSE handshake** — `/sse` returns `event: endpoint` with session ID
- **Verified health endpoint** — `GET /health` returns `{"status":"ok"}`
- **Verified LightRAG API** — health check returns full config (version 1.4.9.4)
- **Verified WebUI** — serves at `http://localhost:9621/`, returns 200
- **Added `coderag mcp <dir>` subcommand** — stdio↔SSE bridge for opencode integration
- **cppencode bridge verified** — `initialize` + `tools/list` return all 22 tools via stdio
- **opencode config updated** — `coderag-mcp` uses `type: "local"` with `command: ["coderag", "mcp", "."]`
- **Backup saved** — `~/.config/opencode/opencode.json.backup.*`

### Tool Reduction (June 2026) ✅
- **Reduced MCP tool set from 22 to 6 focused tools** for AI agent codebase exploration
- **6 tools kept:** `query`, `get_related_files`, `check_entity_exists`, `get_knowledge_graph`, `get_graph_labels`, `add_note_to_entity`
- **Removed:** all document management tools (insert_text, insert_texts, upload_document, scan_documents, get_documents, delete_document, etc.)
- **`add_note_to_entity` implemented** — creates embedded NOTE entity linked via HAS_NOTE relationship
  - Takes `entity_name` and `note_text` parameters
  - Checks entity exists, creates NOTE entity with description=note_text (LightRAG embeds it), creates HAS_NOTE relation
  - Returns `AddNoteResponse` with status, entity_name, note_entity_name, relationship_created

## Architecture

```
opencode ──stdio──► coderag mcp . (host Go binary)
                        │
                   SSE bridge ──HTTP──► mcp (Docker :9622)
                                            │
                                       talks to lightrag (Docker :9621)
```

| Layer | Component | Location | Transport |
|-------|-----------|----------|-----------|
| 1 | opencode | Host (terminal) | stdio ↔ coderag |
| 2 | `coderag mcp .` | Host (Go binary) | stdio↔SSE bridge |
| 3 | Python MCP server | Docker (:9622) | SSE |
| 4 | LightRAG API | Docker (:9621) | HTTP |

- `coderag mcp <dir>` ensures Docker is running, connects to SSE, bridges stdio↔SSE
- MCP and watchdog are **independent** — both talk to LightRAG API only
- No Ollama in Docker — assumed reachable on host at `http://localhost:11434`
- `--workers 1` required for LightRAG server

## Verification Steps

1. `docker compose build` — builds image with WebUI + LightRAG + MCP
2. `docker compose up -d` — starts both services
3. `coderag mcp <dir>` — starts stdio↔SSE bridge (test with `tools/list`)
4. `coderag start <dir>` — starts Docker + file watcher
5. `coderag query "what classes?"` — returns entities
6. `coderag webui` — opens browser
7. `coderag status` — shows healthy
8. `coderag stop` — shuts down containers

---

# Phase 22 — MCP Tool Improvements for Large Codebases 🔧 IN PROGRESS

## Goal

Make the MCP tool set genuinely useful for large, unfamiliar codebases. Fix broken tools, add graph traversal and entity browsing capabilities.

## Root Cause

`get_related_files` calls `POST /query` (LLM summarization endpoint) which returns `response` (LLM text) + `references` (file paths). The method expects `query_result.results` which is always `None` — so it always returns empty.

**Fix:** Use `POST /query/data` which returns structured `{entities, relationships, chunks, references}`.

## Gap Analysis

| Need | Current Tool | Status |
|------|-------------|--------|
| Orientation — "What's in this codebase?" | `get_graph_labels` | ✅ Works |
| Finding things — "Find authentication code" | `get_related_files` | ❌ Broken |
| Navigation — "Show me class X's methods" | (missing) | ❌ No tool |
| Relationships — "What calls this method?" | (missing) | ❌ No tool |
| Overview — full graph dump | `get_knowledge_graph` | ⚠️ Too large |
| Filtering by type/file | (missing) | ❌ No tool |
| Pattern search | (missing) | ❌ No tool |

## Tool Changes

### Fix Existing
1. **`get_related_files`** — Fix to use `/query/data` endpoint, return structured results with file paths and snippets

### New Exploration Tools
2. **`query_data`** — Structured query using `/query/data`. Returns entities + relationships + references (no LLM text).
3. **`list_entities_by_type`** — Paginated list of CLASS/METHOD/PROPERTY entities. Uses `/graphs?label=TYPE`.
4. **`get_entities_in_file`** — Filter graph by file_path. Parse graph nodes, filter by `properties.file_path`.
5. **`search_entities`** — Regex search on entity names/descriptions. Uses `/query/data` with pattern in query text.

### New Graph Traversal
6. **`get_entity_neighbors`** — 1-hop neighbors of an entity (all directly connected entities via any relation type).
7. **`get_incoming_relations`** — Entities that point TO this entity (relation.source → this entity).
8. **`get_outgoing_relations`** — Entities this entity points TO (this entity → relation.target).
9. **`get_entity_details`** — Full entity info with all its relations (node + adjacent edges).

### New Overview
10. **`get_codebase_stats`** — Summary: entity counts by type, relation counts by type, total files, total chunks.

## Implementation Details

### `/query/data` response structure
```json
{
  "status": "success",
  "data": {
    "entities": [{"entity_name": "...", "entity_type": "CLASS", "description": "...", "file_path": "...", ...}],
    "relationships": [{"src_id": "...", "tgt_id": "...", "relation_type": "...", ...}],
    "chunks": [...],
    "references": [{"file_path": "...", "chunk_id": "..."}]
  }
}
```

### Graph traversal approach
For neighbor/relation queries, fetch `/graphs` (full graph) once, then filter locally:
- `get_entity_neighbors`: find all edges where src_id or tgt_id = entity_name, collect the other side
- `get_incoming_relations`: find edges where tgt_id = entity_name
- `get_outgoing_relations`: find edges where src_id = entity_name

### Entity type filtering
Use `/graphs?label=CLASS` to get only CLASS entities. Valid labels from `get_graph_labels`.

### File path filtering
Graph nodes have `properties.file_path`. When filtering, compare normalized paths (handle Windows/Unix differences).

## Steps

| # | Step | Status |
|---|------|--------|
| 1 | Fix `get_related_files` to use `/query/data` | ⬜ Pending |
| 2 | Add `query_data` tool | ⬜ Pending |
| 3 | Add `list_entities_by_type` tool | ⬜ Pending |
| 4 | Add `get_entities_in_file` tool | ⬜ Pending |
| 5 | Add `search_entities` tool | ⬜ Pending |
| 6 | Add `get_entity_neighbors` tool | ⬜ Pending |
| 7 | Add `get_incoming_relations` / `get_outgoing_relations` | ⬜ Pending |
| 8 | Add `get_entity_details` tool | ⬜ Pending |
| 9 | Add `get_codebase_stats` tool | ⬜ Pending |
| 10 | Update models.py with new request/response types | ⬜ Pending |
| 11 | Rebuild Docker image | ⬜ Pending |
| 12 | Verify all tools work with real codebase | ⬜ Pending |
| 13 | Update documentation | ⬜ Pending |

## Final Tool Set (12 tools)

| Tool | Purpose |
|------|---------|
| `query` | LLM-powered natural language query |
| `query_data` | Structured query without LLM (entities + references) |
| `get_related_files` | Find files by semantic search (FIXED) |
| `get_knowledge_graph` | Full graph (with pagination consideration) |
| `get_graph_labels` | List entity/relation types |
| `add_note_to_entity` | Attach embedded note to entity |
| `list_entities_by_type` | Browse entities by type |
| `get_entities_in_file` | Filter by file path |
| `search_entities` | Pattern search on entity names |
| `get_entity_neighbors` | 1-hop graph traversal |
| `get_incoming_relations` / `get_outgoing_relations` | Directional relation browsing |
| `get_codebase_stats` | Summary statistics |
