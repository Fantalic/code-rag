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
