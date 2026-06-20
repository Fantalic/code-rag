# Current Context

## Status
- 🔥 **Phase 21** — coderag Restructuring 🔧 IN PROGRESS
- **Phase 20** — VDB Embedding & file_path Persistence ✅ COMPLETE
- **Phase 19** — Incremental Update Pipeline Fixes ✅ COMPLETE

## Phase 21 — coderag Restructuring 🔧 IN PROGRESS

### Architecture Decision (June 2026)

**Goal:** `coderag` becomes the single CLI tool for the entire CodeLightRAG system.

**Big changes from previous architecture:**
1. **`coderag` is both the project name AND the CLI binary** — replaces `watchdog` as the main entry point
2. **Everything modular in Go** — `pkg/watchdog/`, `pkg/mcp/`, `pkg/lightrag/` are importable packages
3. **Go MCP replaces Python MCP** — `daniel-lightrag-mcp/` will be removed, `pkg/mcp/` with `mark3labs/mcp-go` takes over
4. **Docker-first** — LightRAG + WebUI run in Docker/Podman, never as host subprocess
5. **WebUI served from LightRAG API** — static files built into Docker image
6. **Analysers at root** — `analysers/csharp/` becomes standalone directory

### Why this matters

```
Before:                        After:
  watchdog (Go CLI)            coderag (Go CLI, single binary)
  daniel-lightrag-mcp (Python)   ├── pkg/mcp/ (Go MCP, 22 tools)
  LightRAG (Python Docker)        ├── pkg/watchdog/ (file watcher)
  WebUI (React, host-only)        ├── pkg/lightrag/ (HTTP client)
                                  └── cmd/coderag/main.go (CLI entry)

                              Docker:
                                ├── LightRAG API + WebUI
                                └── Ollama (optional sidecar)
```

### Implementation steps

| # | Step | Status |
|---|------|--------|
| 1 | Move `watchdog/internal/*` → `pkg/watchdog/` | ⏳ Pending |
| 2 | Create `pkg/lightrag/` HTTP client | ⏳ Pending |
| 3 | Create `pkg/mcp/` with mark3labs/mcp-go (22 tools) | ⏳ Pending |
| 4 | Create `cmd/coderag/main.go` CLI | ⏳ Pending |
| 5 | Wire `coderag mcp` → Docker + watcher + MCP loop | ⏳ Pending |
| 6 | Wire `coderag start` → Docker + watcher | ⏳ Pending |
| 7 | Wire `coderag webui` → open browser | ⏳ Pending |
| 8 | Standalone `watchdog` entry point (thin) | ⏳ Pending |
| 9 | Remove `daniel-lightrag-mcp/` | ⏳ Pending |
| 10 | Move `analysers/` to root | ⏳ Pending |
| 11 | Update go.mod, Dockerfile, docker-compose.yml | ⏳ Pending |
| 12 | Update .opencode/opencode.json | ⏳ Pending |

### Key CLI behavior spec

```
coderag mcp .
  → docker compose up -d (if LightRAG not healthy at :9621)
  → wait for health (poll 2s, timeout 60s)
  → start file watcher on .
  → enter MCP stdio loop

coderag start .
  → same Docker check
  → start file watcher in foreground
  → block until Ctrl+C

coderag webui
  → open http://localhost:9621 in browser

coderag status
  → check LightRAG health + watcher status

coderag stop
  → docker compose down

coderag query "<text>"
  → POST /query to LightRAG, print response
```

### Docker + Podman

- First checks for Docker (`docker compose`), falls back to Podman (`podman-compose`)
- Detection via `DOCKER_HOST` env var or `PATH` lookup
- No direct subprocess fallback for LightRAG

---

## Previous Phase Summary

### Phase 20 — VDB Embedding & file_path Persistence ✅ COMPLETE

- `aedit_entity()` and `aedit_relation()` now include `file_path` in VDB upsert
- Previously, ActionUpdate path overwrote VDB entries without `file_path`
- 11 new integration tests added
- All 74 root tests pass

### Phase 19 — Incremental Update Pipeline Fixes ✅ COMPLETE

- `RelationKey` sorts src/tgt to match LightRAG's undirected edge storage
- CLASS entity update filter narrowed (only `description`/`methods`)
- ParseCSharpFile rewritten with per-class accumulator
- 42 entities, 45 relations, zero errors

### Phase 18 — Project-Based Storage ✅ COMPLETE

- `.coderag/rag_storage` per project, auto-detected via `find_project_root()`
- GraphML NoneType filter in `networkx_impl.py`
- No `.env` file required

---

## What Exists

### Running system
```
coderag (Go MCP + watcher) ──HTTP──► LightRAG API (Docker) ──► Knowledge Graph
                                              ▲
                                     WebUI served from LightRAG
```

### coderag CLI (in progress)
- Not yet built — this is the restructuring goal

### Go watchdog (current)
- `watchdog/cmd/watchdog/main.go` — CLI with `watch`, `start`, `status`, `stop`, `query`
- `watchdog/internal/` — watcher, diff, pipeline, parser, router, filter, crud
- Initial sync on startup — full scan, solution-level analysis, diff-based CRUD
- Git-based change detection (30s poll) — `git diff --name-only HEAD`
- File hash tracking (MD5) — skips unchanged content
- C# analyser: extracts CLASS, METHOD, PROPERTY, INHERITS_FROM, DEPENDS_ON, CALLS

### Python MCP server (to be removed)
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/` — 22 MCP tools in Python
- Will be replaced by Go `pkg/mcp/`

### Test project (`../test-csharp-project/`)
- Separate git repo outside code-lightrag
- `.coderag/rag_storage/` with LightRAG data (graphml + vector DB)
- Source files: Calculator, AdvancedCalculator, DataStore, ExtendedTypes, Class1

---

## Key Technical Decisions

1. **Go single binary on host** — coderag is a native binary (~20MB) built on user's machine for Windows/Linux/macOS
2. **Docker/Podman for services** — LightRAG + WebUI always in containers
3. **Go MCP replaces Python MCP** — `mark3labs/mcp-go` library handles protocol
4. **Modular packages** — `pkg/watchdog/`, `pkg/mcp/`, `pkg/lightrag/` can be imported independently
5. **No API key auth** — local-only Ollama, no cloud dependencies
6. **Project-based storage** — `.coderag/rag_storage` per project, auto-detected
7. **WebUI served by LightRAG** — static files in Docker image, no separate server needed
8. **Analysers at root** — `analysers/csharp/` standalone, callable from CLI, watcher, or WebUI
9. **Entity-level CRUD** > document pipeline — no LLM calls for analyser output
10. **Security filter built into pipeline** — secret data never reaches graph

## Learnings

### Event loop trap
Session-scoped async fixtures + function-scoped test event loops = deadlock. Fix: `asyncio_default_test_loop_scope = session`.

### LightRAG dedup
`POST /documents/text` with existing `file_source` returns `status="duplicated"` and does nothing.

### GraphML NoneType
NetworkX's GraphML writer rejects `None` attribute values. Must filter before every write.

### Undirected graph relation key sorting
LightRAG stores edges with sorted `source`/`target`. Diff engine `RelationKey()` must sort both sides.

### CLASS entity update skip is too broad
Skip only `description`/`methods`, not the entire entity. `file_path` needs to update.

### VDB upsert replaces entirely
NanoVectorDB `upsert()` replaces the full entry. Every field must be included in every upsert, not just changed fields.
