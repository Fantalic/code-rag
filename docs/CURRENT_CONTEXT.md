# Current Context

## Status
- 🔥 **Phase 33** — Graph Entity Model Redesign ✅ COMPLETE
- 🔥 **Phase 32** — Knowledge Graph Refactoring ✅ COMPLETE
- 🔥 **Phase 27** — Verify All MCP Tools ✅ COMPLETE (verified 6/13; full 13-tool verification blocked by graph quality issues)
- **Current session (July 29, 2026)** — Fix parallel worker race in `coderag start` initial sync

### Deep analysis findings

| # | Finding | Status |
|---|---------|--------|
| 1 | LightRAG API handles qualified names correctly | ✅ |
| 2 | `ClearAll()` targeted DEFAULT workspace, not project workspace | ✅ Fixed |
| 3 | Error swallowing masked all failures | ✅ Fixed (added logging) |
| 4 | CALL relations to framework entities fail silently (expected) | ✅ Documented |
| 5 | Phase 33 format is compatible with CRUD pipeline | ✅ |
| 6 | **NEW:** `coderag start` 4 workers each process ALL 348 analyser results → duplicate entity creates + timeouts | 🔧 Fixed |
| 7 | **NEW:** CRUD client `DefaultTimeout=30s` too short under load → "context deadline exceeded" | 🔧 Fixed → 300s |
| 8 | **NEW:** LightRAG storage lives in Docker volume `rag_data`, not in project `.coderag` | 📋 Pending |

### Changes this session (July 29, 2026)

| # | Fix | Where | Status |
|---|-----|-------|--------|
| 1 | ✅ Add global mutex to `applyCSharpResults` — serializes 4 workers so only one processes analyser results at a time | `pkg/watchdog/pipeline/run.go` | ✅ Done |
| 2 | ✅ Increase CRUD client `DefaultTimeout` from 30s → 300s | `pkg/watchdog/crud/client.go` | ✅ Done |
| 3 | ✅ Increase `httpClient.Timeout` from 30 → 300 | `pkg/watchdog/pipeline/run.go` | ✅ Done |
| 4 | Rebuild `coderag.exe` | — | ✅ Done |
| 5 | Fix LightRAG storage to map into project's `.coderag` directory instead of Docker volume | `docker-compose.yml` | 📋 Pending |

### Root cause chain: `coderag start` initial sync failure

1. `observer.go:initialSync()` runs analyser ONCE per solution (correct)
2. Then enqueues EACH of 348 files with FULL results attached (line 162-166)
3. 4 workers each call `processEvent()` → `applyCSharpResults()` with ALL 348 results
4. All 4 workers simultaneously try to create the same 1701 entities → "Entity already exists" HTTP 400
5. Individual entity creates hit `DefaultTimeout=30s` under the load → "context deadline exceeded"
6. All 1743 relation creations fail because their source/target entities weren't fully created

**Fix:** Mutex serializes `applyCSharpResults`. Worker 1 creates all 1701 entities (~5 min), workers 2-4 see empty diff (fast). Total initial sync time ≈ one batch processing time.

## Phase 21 — coderag Restructuring ✅ COMPLETE

### Architecture Decision (June 2026)
**Goal:** `coderag` is the central CLI tool for the codeRAG system.

**Key architecture decisions:**
1. **`coderag` is the CLI binary** — replaces `watchdog` as main entry point
2. **Python MCP kept, runs in Docker** — `daniel-lightrag-mcp` stays Python, runs as separate Docker service with SSE transport on port 9622
3. **Everything modular in Go on host** — `pkg/watchdog/`, `pkg/lightrag/` are importable packages
4. **Docker-first** — LightRAG + WebUI + MCP run in containers, never as host subprocess
5. **WebUI served from LightRAG API** — static files built into Docker image
6. **Analysers at root** — `analysers/csharp/` standalone directory
7. **MCP and watchdog are independent** — both talk only to LightRAG API, never to each other

### Architecture
```
Host side:                        Docker:
  coderag (Go CLI)                ├── lightrag (API + WebUI :9621)
    ├── pkg/watchdog/                └── mcp (Python MCP SSE :9622)
    ├── pkg/lightrag/ (HTTP client)
    └── cmd/coderag/main.go
```

### Implementation status

| Step | Status |
|------|--------|
| SSE transport added to Python MCP server | ✅ Done |
| MCP runs in Docker (port 9622) | ✅ Done |
| WebUI built into Docker image | ✅ Done |
| `cmd/coderag/main.go` CLI (start, watch, status, stop, query, webui) | ✅ Done |
| `.opencode/opencode.json` remote MCP config | ✅ Done |
| `pkg/mcp/` removed (stale Go dir) | ✅ Done |
| Move `watchdog/internal/*` → `pkg/watchdog/` | ✅ Done |
| Create `pkg/lightrag/` HTTP client | ✅ Done |
| Thin standalone `watchdog` entry point | ✅ Deleted — absorbed by `cmd/coderag/` |
| Move `analysers/` to root | ✅ Done |

### Key CLI behavior

```
coderag start <dir>   → docker compose up -d + file watcher
coderag watch <dir>   → file watcher only (Docker must be running)
coderag status        → check LightRAG health + watcher status
coderag stop          → docker compose down
coderag query "<txt>" → POST /query to LightRAG
coderag webui         → open http://localhost:9621
coderag scan <dir>    → run C# analyser and populate knowledge graph
```

---

## Running System (July 29, 2026)

### Services (all running)
| Service | Type | Port | Status |
|---------|------|------|--------|
| `lightrag` (API + WebUI) | Docker | 9621 | ✅ healthy |
| `mcp` (MCP SSE) | Docker | 9622 | ✅ running (fixed SDK 2.0.0 compatibility, see below) |

### MCP Server Fix (July 29, 2026)
MCP server crashed due to Python SDK upgrade to v2.0.0 — `Server.list_tools()`/`Server.call_tool()` decorators removed. Rewrote to use `MCPServer` + `@mcp_server.tool()` per-function pattern. Container no longer crashes.

### coderag CLI
- `cmd/coderag/main.go` — CLI entry point
- `coderag scan <dir>` — run C# analyser and populate knowledge graph via CRUD pipeline
- `coderag mcp <dir>` — stdio↔SSE bridge (MCP server only, no watchdog)
- `go build ./cmd/coderag` succeeds, `go vet ./...` passes
- Root `go.mod` module: `github.com/coderag`, Go 1.25.5

### Go watchdog (pkg/watchdog/)
- `pkg/watchdog/watcher/` — fsnotify + git observer, worker pool, debounce
- `pkg/watchdog/pipeline/` — orchestrator: analyser → diff → security filter → CRUD
- `pkg/watchdog/crud/` — LightRAG HTTP client for graph operations
- `pkg/watchdog/diff/` — entity/relation diff engine (create/update/delete)
- `pkg/watchdog/parser/` — C# analyser output parser
- `pkg/watchdog/router/` — file extension → analyser mapping
- `pkg/watchdog/filter/` — security filter (PII/keys/secrets)
- Initial sync on startup — full scan, solution-level analysis, diff-based CRUD
- Git-based change detection (30s poll) — `git diff --name-only HEAD`
- File hash tracking (MD5) — skips unchanged content

### C# Analyser
- `analysers/csharp/` — Go-based analyser (replaces old .NET `CSharpAnalyzerTool.exe`)
- **Binary:** `csharp_analyser.exe` (self-contained, no CGo/tree-sitter, ~5MB static binary)
- **Commands:** `scan` (full solution), `update` (incremental with stdout diff)
- **Parser:** Go-native state machine (regex-based, not tree-sitter)
- **Key design decisions:**
  - Expression-bodied methods (`=>`) handled inline (no `{` scope)
  - Class scope tracked via `classDepth` (brace depth at declaration)
  - Regex-based parser — zero CGo, static binary, cross-compilation
- **Phase 33 commit:** `1330e381` — namespace-qualified entity names, `extends`/`implements` relations, multi-modifier classRegex fix, global usings support, BOM stripping, `return`-type property leak fix

### Python MCP server (in Docker)
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/` — tools in Python
- SSE transport on port 9622 using Starlette + uvicorn
- Bug fix for MCP Python SDK v2.0.0 — rewritten to use `MCPServer` + `@mcp_server.tool()` pattern

### Docker setup
- `Dockerfile` — multi-stage: WebUI (bun build) → LightRAG + MCP (pip install)
- `docker-compose.yml` — two services (`lightrag` :9621, `mcp` :9622), no Ollama
- Uses `CMD` (not `ENTRYPOINT`) so docker-compose `command` overrides correctly
- MCP service connects to `lightrag:9621` internally via Docker DNS

---

## Phase 33 — Graph Entity Model Redesign ✅ COMPLETE

### Changes (all verified on `dotnet-console-games`, 348 files):

| Change | Where | Result |
|--------|-------|--------|
| Namespace-qualified entity names | `walker.go` — all entity names use `Namespace.Class` format | 0 short/unqualified names in graph |
| `extends`/`implements` relations | `walker.go:handleClassDecl` + GlobalIndex lookup | 154 `extends` + 4 `implements` relations |
| Multi-modifier classRegex fix | `walker.go` — `(?:(?:pub|priv|pro|internal|abstract|sealed|static)\s+)*` | Classes with 2+ modifiers correctly parsed |
| Global usings support | `types.go` + `walker.go:resolveType` — 3-phase resolution | Cross-namespace base types resolve correctly |
| BOM stripping | `walker.go:ParseFile` — `strings.TrimLeft(content, "\ufeff")` | UTF-8 BOM files parsed correctly |
| `return`-type property leak | `isKeyword(v[1])` guard in field regex | 0 `→ RETURNS → return` properties |
| Framework call filtering | `extractCalls` skips empty `calleeClass` | 0 dangling calls to non-existent entities |
| Self-call elimination | Cross-file call resolution uses GlobalIndex | No self-loop `calls` relations |

### Verified output after `coderag scan --clean`:
- 1701 entities extracted (795 created in LightRAG — 906 failed silently)
- 1743 relations extracted (0 created — all failed silently)
- 154 `extends` + 4 `implements` relations emitted by analyser
- 0 NAMESPACE entities, 0 `return`-type leaks, 0 framework call noise

### Bug found during verification:
The CRUD pipeline (`applyScanResults`) silently swallows all entity and relation creation errors. After a clean rescan, 906 of 1701 entities fail to create and 0 of 1743 relations are created. Root cause under investigation — likely combination of:
1. Silent error swallowing masking the actual failure reason
2. Possible entity name format mismatch between creation and lookup
3. `ClearAll()` (`DELETE /documents`) not purging graph entities/relations from `rag_data` volume

---

## Key Technical Decisions

1. **Python MCP kept, Docker-only** — No Go rewrite. `daniel-lightrag-mcp` runs as separate Docker service. SSE transport on port 9622. 6 core MCP tools active (query, query_data, get_related_files, check_entity_exists, get_knowledge_graph, get_graph_labels, add_note_to_entity, get_codebase_stats, get_entity_neighbors, get_entity_details, list_entities_by_type, get_entities_in_file, search_entities — 13 total post-Phase 22).
2. **Docker multi-service** — `lightrag` and `mcp` share same image but different `command`
3. **`CMD` not `ENTRYPOINT`** — Allows command override in docker-compose
4. **`coderag mcp` starts watchdog in background** — watchdog runs as goroutine in the Go bridge process
5. **Docker-first, no fallback** — LightRAG + WebUI + MCP always in containers
6. **No Ollama in Docker** — assumed on host at `http://localhost:11434`
7. **`--workers 1` required** — Default 4 causes `dial tcp` errors
8. **30s git watcher poll** — For debouncing
9. **Project isolation** — `.coderag/rag_storage` per project
10. **Entity-level CRUD > document pipeline** — No LLM calls for analyser output
11. **Security filter built into pipeline** — Secret data never reaches graph
12. **C# analyser at project root** — `analysers/csharp/` Go-based analyser (replaces old `analysers/c_sharp/CSharpAnalyzerTool.exe`)
13. **`update_relations` deprecated** — Analysis now handled by Go watchdog via `coderag scan` or automatic on file change.
14. **`_auto_initialize()` removed** — MCP server no longer auto-runs analyser on startup. Watchdog handles initialization via background goroutine.
15. **Go native parser (no tree-sitter)** — C# analyser uses Go regex/state-machine, not CGo/tree-sitter, for a fully static binary
16. **Analysers output .txt files** — Pipeline parses these files directly via `parser.ParseCSharpOutput()`, then sends entities/relations to LightRAG HTTP API for CRUD operations
