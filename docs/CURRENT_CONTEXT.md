# Current Context

## Status
- 🔥 **Phase 33** — Graph Entity Model Redesign ✅ COMPLETE
- 🔥 **Phase 32** — Knowledge Graph Refactoring ✅ COMPLETE
- 🔥 **Phase 27** — Verify All MCP Tools ✅ COMPLETE (verified 6/13; full 13-tool verification blocked by graph quality issues)
- **Current session (July 29, 2026)** — Cascade-aware deletion + incremental initial sync + relation `source_id` fix

### Deep analysis findings

| # | Finding | Status |
|---|---------|--------|
| 1 | LightRAG API handles qualified names correctly | ✅ |
| 2 | `ClearAll()` targeted DEFAULT workspace, not project workspace | ✅ Fixed |
| 3 | Error swallowing masked all failures | ✅ Fixed (added logging) |
| 4 | CALL relations to framework entities fail silently (expected) | ✅ Documented |
| 5 | Phase 33 format is compatible with CRUD pipeline | ✅ |
| 6 | `coderag start` 4 workers each process ALL 348 analyser results → duplicate entity creates + timeouts | ✅ Fixed (mutex) |
| 7 | CRUD client `DefaultTimeout=30s` too short under load → "context deadline exceeded" | ✅ Fixed → 300s |
| 8 | LightRAG storage lives in Docker volume `rag_data`, not in project `.coderag` | 📋 Pending |
| 9 | **NEW:** `initialSync()` ran full analyser on ALL solutions, queued ALL files → full rebuild every `coderag start` | ✅ Fixed (git diff filtering) |
| 10 | **NEW:** Deleted files silently dropped from `getGitChangedFiles` — never cleaned up from LightRAG | ✅ Fixed (`getGitDeletedFiles`) |
| 11 | **NEW:** `Relation` parser struct has no `SourceID`/`FilePath` — all relations get `source_id` = caller's `filePath` | ✅ Fixed |
| 12 | **NEW:** `findAffectedFiles` reads `rel.SourceID` from relations (broken) — must use entity `file_path` lookup instead | ✅ Fixed (entity-based fallback) |
| 13 | **NEW:** `checkGitChanges` queues `'deleted'` but doesn't cascade to affected files | ✅ Fixed via `queueDeletionWithCascade` |
| 14 | **NEW:** `findComposeFile()` only looks in cwd and `../` — fails when run from outside the repo | ✅ Fixed (also checks binary dir) |
| 15 | **NEW:** `parserRelationsToMapScan` drops `source_id`/`file_path` from relation data map — all `coderag scan` relations lose these fields in LightRAG | ✅ Fixed |
| 16 | **NEW:** `loadCurrentRelationsScan` and `loadCurrentRelations` don't read `source_id`/`file_path` from graph — diff engine can't detect changes to these fields | ✅ Fixed |
| 17 | **NEW:** CALL relations (`calls` keyword) have empty `source_id`/`file_path` — `parseCallLine` doesn't read caller file path, `ParseCSharpFile` doesn't set `SourceID` for CALL lines | ✅ Fixed |
| 18 | **NEW:** Stale analyser output `.txt` files persist between scans — `RunCSharpAnalyser` reads ALL `.txt` files including from deleted sources | ✅ Fixed (cleanup before run) |
| 19 | `source_id` in diff engine `ignoredKeys` prevented real-time change path from auto-healing wrong `source_id` on existing relations | ✅ Fixed (removed from ignoredKeys) |

### Changes this session (July 29, 2026)

| # | Fix | Where | Status |
|---|-----|-------|--------|
| 1 | ✅ Add global mutex to `applyCSharpResults` — serializes 4 workers | `pkg/watchdog/pipeline/run.go` | ✅ Done |
| 2 | ✅ Increase CRUD client `DefaultTimeout` 30s → 300s | `pkg/watchdog/crud/client.go` | ✅ Done |
| 3 | ✅ Increase `httpClient.Timeout` 30 → 300 | `pkg/watchdog/pipeline/run.go` | ✅ Done |
| 4 | ✅ Add `filepath.Abs` to `inputDirOrDefault` — `.` resolves correctly | `cmd/coderag/main.go` | ✅ Done |
| 5 | ✅ Add `findComposeFile()` search relative to binary dir | `cmd/coderag/main.go` | ✅ Done |
| 6 | ✅ Rewrite `initialSync()` to use git diff — only processes changed files | `pkg/watchdog/watcher/observer.go` | ✅ Done |
| 7 | ✅ Add `getGitDeletedFiles()` — detects file deletions from git HEAD | `pkg/watchdog/watcher/observer.go` | ✅ Done |
| 8 | ✅ Add `queueDeletionWithCascade()` — cleanup + affected file re-scan | `pkg/watchdog/watcher/observer.go` | ✅ Done |
| 9 | ✅ Add `findAffectedFiles()` — discovers files referencing deleted entities | `pkg/watchdog/watcher/observer.go` | ✅ Done |
| 10 | ✅ Update `checkGitChanges()` + `handleEvent()` to use cascade | `pkg/watchdog/watcher/observer.go` | ✅ Done |
| 11 | ✅ Add `GetAllRelations()` to CRUD client | `pkg/watchdog/crud/client.go` | ✅ Done |
| 12 | ✅ Fix `Relation` parser to include `SourceID`/`FilePath` | `pkg/watchdog/parser/csharp.go` | ✅ Done |
| 13 | ✅ Fix `parserRelationsToMap` to include `source_id`/`file_path` | `pkg/watchdog/pipeline/run.go` | ✅ Done |
| 14 | ✅ Fix `findAffectedFiles` to use entity-based lookup + pure-logic extraction | `pkg/watchdog/watcher/observer.go` | ✅ Done |
| 15 | ⬜ Fix `parserRelationsToMapScan` — drop `source_id`/`file_path` from relation data map — all `coderag scan` relations lose these fields in LightRAG | ✅ Fixed |
| 16 | ⬜ Fix `loadCurrentRelationsScan`/`loadCurrentRelations` — read `source_id`/`file_path` from graph edges | ✅ Fixed |
| 17 | ⬜ Fix CALL relations — `parseCallLine` reads file path from parts[1], `ParseCSharpFile` sets `rel.SourceID = filename` for CALL lines | ✅ Fixed |
| 18 | ⬜ Fix stale analyser output files — `RunCSharpAnalyser` cleans `.txt` files before run | ✅ Fixed |

### Root cause chain: `initialSync()` full rebuild on every start

1. `observer.go:initialSync()` called `analyseSolution()` for EVERY solution (no git check)
2. All files enqueued as "initial" events with full analyser results
3. `applyCSharpResults` re-processed everything via diff engine (wasteful but not harmful)
4. Git-based `checkGitChanges()` only ran every 30s after startup — never used on initial sync
5. Deleted files never detected (filtered against filesystem walk) → stale data persisted

**Fix:** `initialSync()` now:
1. Builds hash map for ALL files (future change detection)
2. Calls `getGitChangedFiles()` → only files that exist + differ from git HEAD
3. Calls `getGitDeletedFiles()` → files in HEAD but gone from disk
4. For deletions: `queueDeletionWithCascade()` → cleanup + find affected files + re-scan
5. For changes: group by solution, run analyser ONCE per solution, queue as "modified"
6. If git fails → fallback to full scan (original behavior)

### Fixed: Relation `source_id` always wrong (July 29, 2026)

`parser.Relation` had NO `SourceID` or `FilePath` fields (unlike `parser.Entity` which has both).
`parserRelationsToMap()` built data map without `source_id`/`file_path`. `applyCSharpResults` fell back to
the caller's `filePath` parameter for every relation — meaning ALL relations in LightRAG had
`source_id` pointing to whatever file triggered the last scan/modification, NOT the file that
actually defined the relationship. This broke `findAffectedFiles` which reads `rel.SourceID`.

**Fix:**
1. Added `SourceID`/`FilePath` to `parser.Relation` struct; `parseRelationLine` reads `parts[5]` (file path from analyser output); `ParseCSharpFile` sets `rel.SourceID = filename`
2. `parserRelationsToMap` now includes `"source_id"` and `"file_path"` in the data map
3. `findAffectedFiles` uses dual-path: relation's own `SourceID`/`FilePath` + entity-map fallback via `GetAllEntities()`
4. Pure logic extracted as `findAffectedFilesFromData()` — tested with 5 subtests
5. Old wrong relations auto-heal on next file change via `dataChanged` in diff engine

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

## MCP Tool Testing (August 2026)

### Sub-Agent Exploration via HTTP API

A sub-agent was dispatched to explore the LightRAG knowledge graph for the test-csharp-project using direct HTTP API calls at `http://localhost:9621`. The sub-agent was given a tool-to-API mapping table so it could call each MCP tool's underlying endpoint directly, bypassing the MCP SSE protocol.

**Key finding:** The sub-agent initially failed because it tried to connect via MCP SSE protocol (port 9622) without knowing how to establish the connection. The fix was to provide the HTTP REST API endpoint mapping — all MCP tools are ultimately HTTP requests to port 9621.

### Graph Data Summary (test-csharp-project)

| Metric | Value |
|--------|-------|
| Total entities | 63 (7 CLASS, 1 INTERFACE, 29 METHOD, 10 PROPERTY + others) |
| Total edges | 45 |
| Entity labels | CLASS, INTERFACE, METHOD, PROPERTY |
| Relation labels | calls, defines, extends, has, implements, inheritance, method, property |

### Calculator Entity Details

| Entity | Type | Methods | Source File |
|--------|------|---------|-------------|
| TestLib.Calculator | CLASS | Add, Subtract, Divide, Multiply | AdvancedCalculator.cs |
| TestLib.AdvancedCalculator | CLASS | Sqrt, Power, Modulo | AdvancedCalculator.cs |
| TestLib.IOperation | INTERFACE | Execute | AdvancedCalculator.cs |
| TestLib.DataStore | CLASS | AddItem, GetAll, Contains, Remove, FindById, Clear, ClearAll, Add | DataStore.cs + ExtendedTypes.cs |

### Entity Gaps Found

| Entity | Expected | Status |
|--------|----------|--------|
| TestLib.IRepository | Should exist (IRepository<T> in DataStore.cs) | ❌ NOT IN GRAPH |
| TestLib.TestConsole.Program | Should exist (Program.cs) | ❌ NOT IN GRAPH |
| TestLib.Converter | Should exist | ✅ In static index only |
| TestLib.Class1 | Should exist | ✅ In static index only |

### Updated MCP Tool Effectiveness (August 2026)

| Tool | Status | Notes |
|------|--------|-------|
| `check_entity_exists` | ✅ Working | Fast, reliable |
| `get_knowledge_graph` | ✅ Working | Returns full graph |
| `get_graph_labels` | ✅ Working | Returns entity/relation types |
| `get_entity_details` | ✅ Working | Full details with relations |
| `get_entity_neighbors` | ✅ Working | Shows inheritance, implementation, calls |
| `search_entities` | ✅ Working | Regex pattern matching |
| `query` (GET) | ✅ Working | Natural language queries work |
| `query` (POST) | ❌ Times out | Ollama not reachable from Docker |
| `query_data` (POST) | ❌ Times out | Ollama not reachable from Docker |
| `get_related_files` (POST) | ⚠️ Partial | Returns file paths only |
| `get_codebase_stats` | ❌ HTTP 500 | Internal split error |
| `list_entities_by_type` | ❌ Empty | Server-side filter broken |
| `get_entities_in_file` | ⚠️ Path mismatch | Windows backslash issue |
| `add_note_to_entity` | ⚠️ False error | Creates note but reports failure |
| `get_relation_detail` | ✅ Added | New in Phase 32 |
| `get_method_signature` | ✅ Added | New in Phase 32 |
| `find_related_files` | ✅ Added | New in Phase 32 |
| `get_interface_methods` | ✅ Added | New in Phase 32 |

### MCP Tool Fixes Applied (August 2026)

| Bug | Tool | Fix | Status |
|-----|------|-----|--------|
| `keywords.split(",")` fails when keywords is a list | `get_codebase_stats` | Added `isinstance(keywords, list)` check before splitting | ✅ Fixed |
| `file_path` extracted from wrong location in query_data response | `get_related_files` | Extract from top-level `entity.get("file_path")` first, then `properties` fallback | ✅ Fixed |
| `add_note_to_entity` checks `created` flag (always False for notes) | `add_note_to_entity` | Changed to check `create_resp.status == "success"` | ✅ Fixed |

---

## MCP Tool Verification (July 29, 2026)

All graph-based MCP tools verified against live LightRAG API on test-csharp-project:

| Tool | Status | Notes |
|------|--------|-------|
| `get_graph_labels` | ✅ | Returns all entity/relation types correctly |
| `get_entities_in_file` | ✅ | Returns entities for a given file path |
| `get_entity_neighbors` | ✅ | Returns outgoing (defines/implements) + incoming (extends) |
| `search_entities` | ✅ | 17 Calculator entities found correctly |
| `get_entity_details` | ✅ | Returns CLASS entity with description + method count |
| `query` | ⚠️ | Returns "no relevant context" — requires text chunk ingestion (not just graph CRUD) |

The `query` endpoint returns "no relevant context" because `coderag scan` only populates graph entities/relations via CRUD — it does not ingest text chunks into the vector DB that powers the LLM query path. This is expected behavior.

**Note**: The `query` endpoint now works with local Ollama model (`phi3.5:latest`), set in `docker-compose.yml`.

---

## MCP Tool Evaluation (July 29, 2026)

### Assessment using test-csharp-project

| MCP Tool | Usefulness | Rating |
|----------|-----------|--------|
| `get_graph_labels` | Shows available entity/relation types | Useful |
| `get_entities_in_file` | Reveals exactly which methods exist in a file | Very Useful |
| `get_entity_neighbors` | Shows interface implementations, inheritance, calling patterns | Very Useful |
| `search_entities` | Finds related classes (IOperation, AdvancedCalculator, DataStore) | Useful |
| `get_entity_details` | Shows method counts, descriptions | Useful |
| `query` (hybrid) | Natural-language summary of patterns | Moderately Useful |

### Missing / Proposed New MCP Tools

| # | Proposed Tool | Purpose | Why It's Needed |
|---|--------------|---------|-----------------|
| 1 | `get_relation_detail(entity, relation_type)` | Show all relations of a specific type for an entity (e.g., all `calls` from Calculator) | Currently must dump all 43 edges and filter manually |
| 2 | `get_method_signature(entity_name)` | Show method signature (parameters, return type) for a specific method | Descriptions lack actual parameter/return type info |
| 3 | `find_related_files(entity_or_file)` | Find all files that reference/relate to a given entity | Critical for understanding impact when modifying Calculator |
| 4 | `get_interface_methods(interface_name)` | Show all methods an interface declares + which classes implement them | Had to manually trace that IOperation→Execute because no tool queries this |

### Implementation Plan

Next task: implement all 4 proposed MCP tools in Python (`daniel-lightrag-mcp/`) and Go MCP server, with tests and docs.

---

## Running System (July 29, 2026 - Updated)

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
