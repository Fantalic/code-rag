# Phase 24 — MCP Tool Return Empty Results Bug Fix ✅ COMPLETE

**Completed previously.** Fixed `get_all_labels()` returning node IDs instead of entity types, added `get_all_entity_types()` and `get_all_relation_types()`, fixed `/graph/label/list` to return structured response.

---

# Phase 25 — Architecture Cleanup & MCP Fixes ✅ COMPLETE

**Completed:**
- C# analyser moved to `analysers/c_sharp/`
- `coderag scan` command added
- Watchdog starts in background on `coderag mcp`
- Old Python watchdog deleted
- `update_relations` deprecated

---

# Phase 26 — Fix Broken MCP Tools 🔧 IN PROGRESS

## Bug Report

From MCP tool testing:
- `get_codebase_stats` returns 0 entities despite graph being populated
- `get_entity_neighbors` and `get_entity_details` fail due to schema mismatches
- Knowledge graph only contains sample Calculator class, not actual project code
- `coderag mcp` doesn't start watchdog, so graph is never populated

## Root Causes

1. **C# analyser in wrong location** — `daniel-lightrag-mcp/analysers/` instead of project root `analysers/`
2. **Edge field name mismatch** — LightRAG returns `source`/`target` but MCP client expects `src_id`/`tgt_id`
3. **Watchdog not started** — `coderag mcp` only starts Docker + bridge, not the file watcher
4. **No explicit scan command** — no way to trigger analysis manually

## Steps

| # | Step | Status |
|---|------|--------|
| 1 | Delete old Python watchdog code (`watchdog/` dir) | ⏳ Pending |
| 2 | Move C# analyser to `analysers/c_sharp/` | ⏳ Pending |
| 3 | Update Go pipeline `findCSharpAnalyser()` path | ⏳ Pending |
| 4 | Fix MCP edge field handling in `get_entity_neighbors` | ⏳ Pending |
| 5 | Fix MCP edge field handling in `get_entity_details` | ⏳ Pending |
| 6 | Add `coderag scan` command | ⏳ Pending |
| 7 | Start watchdog in background on `coderag mcp` | ⏳ Pending |
| 8 | Deprecate `update_relations` (keep, may be used in future) | ⏳ Pending |
| 9 | Remove `_auto_initialize()` from MCP server | ⏳ Pending |
| 10 | Update documentation | ⏳ Pending |
| 11 | Verify with `coderag mcp E:\_dev\_Projects\test-csharp-project` | ⏳ Pending |

---

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

# Phase 22 — MCP Tool Improvements for Large Codebases ✅ COMPLETE

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
| 1 | Fix `get_related_files` to use `/query/data` | ✅ Done |
| 2 | Add `query_data` tool | ✅ Done |
| 3 | Add `list_entities_by_type` tool | ✅ Done |
| 4 | Add `get_entities_in_file` tool | ✅ Done |
| 5 | Add `search_entities` tool | ✅ Done |
| 6 | Add `get_entity_neighbors` tool | ✅ Done |
| 7 | Add `get_incoming_relations` / `get_outgoing_relations` | ✅ Done |
| 8 | Add `get_entity_details` tool | ✅ Done |
| 9 | Add `get_codebase_stats` tool | ✅ Done |
| 10 | Update models.py with new request/response types | ✅ Done |
| 11 | Rebuild Docker image | ✅ Done |
| 12 | Verify all tools work with real codebase | ✅ Done |
| 13 | Update documentation | ✅ Done |

## Final Tool Set (13 tools)

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

---

# Phase 23 — MCP Bug Fixes & Auto-Initialization ✅ COMPLETE

## Bug Report (from agent test)

Agent tested the MCP tools against `E:\_dev\_Projects\test-csharp-project` and found:

| Tool | Status | Issue |
|------|--------|-------|
| `get_codebase_stats` | Works | Shows 0 entities — graph empty |
| `get_graph_labels` | Works | Shows empty labels |
| `query` | Works | Falls back to LLM training data when graph empty → unreliable |
| `search_entities` | Broken | "Internal validation error" |
| `get_entities_in_file` | Broken | Rejects valid file paths |
| `get_related_files` | Fixed | Now uses `/query/data` correctly |

**Root causes identified:**

1. **`get_entities_in_file` bug:** LightRAG stores `file_path` at node level (`node["file_path"]`), not inside `node["properties"]["file_path"]`. Client code looked in wrong place.

2. **`search_entities` bug:** Uses `label="*"` when calling `/graphs`. The `*` is not a valid entity type label — `/graphs` expects a real label like "CLASS".

3. **Empty graph:** MCP server starts without running the C# analyser. Nothing gets indexed automatically.

## Fixes

### Fix 1: `get_entities_in_file` — Correct property location
```python
# WRONG (looks in properties)
node.get("properties", {}).get("file_path", "")
# CORRECT (looks at node level)
node.get("file_path", "")
```
Also add null-check: if `properties` is `None`, treat as empty dict.

### Fix 2: `search_entities` — Valid label for `/graphs`
Instead of `label=entity_type or "*"`, use a valid label. Options:
- Call `get_graph_labels()` first to get the first real label, then use it
- Or fetch without label parameter (but `/graphs` requires `label` query param)
- Best: iterate through available labels from `get_graph_labels()` until one returns entities

### Fix 3: Auto-initialization on MCP startup
- On startup, check current working directory for `*.sln` or `*.csproj` files
- If found, run the C# analyser via `update_relations(path)`
- This populates the graph before any queries arrive
- Project directory = where opencode was started from (cwd)

### Fix 4: Better empty-graph messages
When `get_codebase_stats` returns 0 entities, add a message:
"Knowledge graph is empty. No source files have been indexed yet. Ensure the watchdog is running and watching your project directory."

## Steps

| # | Step | Status |
|---|------|--------|
| 1 | Fix `get_entities_in_file` property lookup | ✅ Done |
| 2 | Add null-check for `properties=None` | ✅ Done |
| 3 | Fix `search_entities` label parameter | ✅ Done (but deeper issue fixed in Phase 24) |
| 4 | Add auto-initialization on startup | ✅ Done |
| 5 | Add better empty-graph messages | ⚠️ Not implemented |
| 6 | Rebuild Docker, verify with real codebase | ✅ Done |
| 7 | Update documentation | ✅ Done |

Note: `search_entities` had a deeper issue — it used `get_knowledge_graph(label=entity_type)` which does BFS from a node named "CLASS", not filtering by type. Fixed in Phase 24.

---

# Phase 26 — Fix Broken MCP Tools ✅ COMPLETE

**All fixes committed to `daniel-lightrag-mcp/src/daniel_lightrag_mcp/client.py` and `server.py`.**

**Fixes applied:**
- Added `_safe_entity_data()` and `_extract_entity_name()` helper functions with fallback chain
- All 4 broken tools now use safe name extraction
- `list_entities_by_type` uses case-insensitive entity_type comparison
- `get_entity_neighbors` uses safe node ID resolution
- `add_note_to_entity` checks both `created` flag AND message content

**Files changed:**
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/client.py` - helper functions + all affected methods
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/server.py` - add_note_to_entity success check

---

# Phase 27 — Verify All MCP Tools ✅ PENDING
