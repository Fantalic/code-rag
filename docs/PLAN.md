# 🗺️ CodeLightRAG — Where We're Going

## The Big Idea

A **live, minimal-work knowledge graph** for codebases. When you save a file, only the affected entities and relations update — no full rebuilds, no wasted tokens, no secrets leaked.

## What We Have ✅

- Monorepo: LightRAG + MCP server + watchdog working together
- 22 MCP tools for AI agents to query the knowledge graph
- C# analyser (Roslyn-based, compiled binary) — extracts classes, methods, inheritance, deps
- File-watch with debouncing + hash tracking (skips no-op events)
- 226 passing tests (63 root + 163 MCP)
- **Go watchdog** (Phase 12-13) — standalone CLI binary replaces Python watchdog

## Phase 11 — Efficient Incremental Updates ✅

All 7 steps done (Python prototype):
- Graph CRUD client (`watchdog/graph_crud.py`)
- Diff engine (`watchdog/graph_diff.py`)
- Security filter (`watchdog/security_filter.py`)
- File router (`watchdog/file_router.py`)
- C# analyser parser (`watchdog/analyser_parsers.py`)
- Pipeline wired in `watchdog/main.py`

## Phase 12 — Go Rewrite ✅

Go watchdog (standalone CLI binary) replacing the Python prototype.

### Go package structure

```
watchdog/
├── cmd/watchdog/main.go       # CLI entry (urfave/cli/v2)
├── go.mod
├── internal/
│   ├── crud/client.go         # Graph CRUD HTTP client
│   ├── diff/engine.go         # Diff engine (EntityAction, RelationAction)
│   ├── filter/security.go     # Security filter (PII/keys/secrets)
│   ├── router/file.go         # Extension -> analyser mapping
│   ├── parser/csharp.go       # C# analyser output parser
│   ├── pipeline/run.go        # Pipeline orchestration
│   └── watcher/observer.go    # File watcher + worker pool
└── Dockerfile
```

### CLI commands

```bash
watchdog watch <dir>            # Watch project folder (connects to existing LightRAG)
watchdog start <dir>            # Start LightRAG + watchdog together
watchdog status                 # Show running services
watchdog stop                   # Stop all services
watchdog query "<text>"         # Query the graph directly
```

### Configuration (env vars + CLI flags)

| Variable | Flag | Default |
|----------|------|---------|
| LIGHTRAG_URL | --url | http://localhost:9621 |
| OLLAMA_HOST | --ollama | http://localhost:11434 |
| INPUT_DIR | positional | — |
| DEBOUNCE_SECONDS | --debounce | 0.5 |
| WATCHER_WORKERS | --workers | 4 |
| LIGHTRAG_API_KEY | --api-key | — |

## Phase 13 — Concurrent Analyser Race Fix ✅

Fixed three issues in the Go watchdog pipeline:

### Issue 1 — Concurrent analyser invocations
- Multiple workers (4 by default) each ran the C# analyser on the same solution independently
- Result: duplicate analyser runs, race conditions, duplicate relation creation errors
- **Fix**: Solution-level mutex (`sync.Mutex` + `chan struct{}` per sln path). Initial sync groups files by solution, runs analyser once, shares results with all workers.

### Issue 2 — LightRAG undirected graph direction swap
- LightRAG uses `nx.Graph()` (undirected). API serializes edges with `source`/`target` alphabetically sorted for consistency
- Relations like `Calculator→Add` appeared as `source=Add, target=Calculator` in graph queries
- **Fix**: `loadCurrentRelations` now reads `src_id`/`tgt_id` from edge `properties` (preserves real creation order)

### Issue 3 — Duplicate relation creation
- Concurrent workers tried to create same relations, LightRAG returned "already exists" errors
- **Fix**: `sync.Map` dedup set in pipeline prevents same relation from being created twice

### Files modified (Phase 13)
- `watchdog/internal/pipeline/run.go` — `AnalyserResult` type, `RunEvent`, relation dedup, fix `loadCurrentRelations`
- `watchdog/internal/watcher/observer.go` — solution grouping, mutex, Event carries pre-computed results
- `watchdog/internal/parser/csharp.go` — METHOD entities now created alongside CLASS entities

### Test result
- C# project: 7 entities (3 CLASS + 4 METHOD), 4 relations (Calculator→Add, Calculator→Subtract, DataStore→AddItem, DataStore→GetAll)
- Directions correct via `src_id`/`tgt_id` from edge properties

### Security

All analysers **must** filter before inserting into the graph:
- 🔴 API keys, tokens, passwords (`sk-...`, `ghp_...`, etc.)
- 🔴 Email addresses, usernames, personal names
- 🔴 IP addresses, internal URLs, server paths
- 🔴 Any `SECRETS`/`PASSWORDS`/`TOKENS` env var values

The security filter lives in the pipeline **after** extraction but **before** graph insertion. This prevents sensitive data from ever reaching the vector DB.

---

## What We're Not Doing (Yet)

- ❌ PyPI publishing
- ❌ Premature corporate docs
- ❌ Full-language support before the pipeline pattern is proven with C#
- ❌ Perfection — experimental project that ships
