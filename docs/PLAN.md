# CodeLightRAG — Where We're Going

## The Big Idea

A **live, minimal-work knowledge graph** for codebases. When you save a file, only the affected entities and relations update — no full rebuilds, no wasted tokens, no secrets leaked.

`coderag` is the single CLI to rule them all. One native binary on the host, Docker/Podman for persistent services, opencode for AI agent access.

---

## What We Have ✅

- Monorepo: LightRAG + WebUI + watchdog + analysers working together
- 22 MCP tools defined (currently in Python `daniel-lightrag-mcp`)
- C# analyser (Roslyn-based, compiled binary) — extracts classes, methods, inheritance, deps
- Go file watcher with debouncing + hash tracking + git poll
- Graph CRUD client, diff engine, security filter, pipeline runner (all in Go)
- Project-based storage (`.coderag/rag_storage`)
- VDB `file_path` persistence fix — entity/relation updates preserve file_path in vector DB
- 74 passing tests

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

## Phase 13 — Concurrent Analyser Race Fix ✅

Solution-level mutex, undirected graph direction fix, duplicate relation dedup.

## Phase 14 — Testing, MCP Tool Usage & Test Expansion ✅

Expanded test-csharp-project with inheritance, interfaces, properties. 168 MCP tests.

## Phase 15 — End-to-End System Testing ✅

Live system test: 50 nodes, 47 edges, queries work. Found entity deletion not working.

## Phase 16 — Fix Entity Deletion & File Change Handling ✅

- Entity deletion confirmed working with JSON body
- `DeleteEntitiesByFilePath()` added to Go watchdog
- Modified files refresh entities correctly

## Phase 17 — Relation Creation Fix ✅

- Router filters non-C# files
- Git watcher path cleaning
- `loadCurrentRelations()` fix for undirected graph edges
- CLASS entity update skip narrowed (only `description`/`methods`)

## Phase 18 — Project-Based Storage & GraphML Fix ✅

- `.coderag/rag_storage` per project, auto-detected
- GraphML NoneType fix
- No `.env` requirement

## Phase 19 — Incremental Update Pipeline Fixes ✅

- `RelationKey` sorts src/tgt
- ParseCSharpFile rewritten (per-class accumulator)
- Modified pipeline re-runs analyser when results empty

## Phase 20 — VDB file_path Persistence ✅

- `aedit_entity()` and `aedit_relation()` now include `file_path` in VDB upsert
- 11 new integration tests
- All 74 root tests pass

---

## Phase 21 — coderag Restructuring 🔧 IN PROGRESS

### Goal

Restructure the monorepo so `coderag` is the central CLI. Everything becomes modular Go packages. The Python MCP server is replaced by Go MCP built into the `coderag` binary.

### Architecture Decision

| Before | After |
|--------|-------|
| `watchdog/` = CLI + watcher | `cmd/coderag/` = CLI, `pkg/watchdog/` = importable watcher |
| `daniel-lightrag-mcp/` (Python MCP) | `pkg/mcp/` (Go MCP, mark3labs/mcp-go) |
| MCP + CRUD logic in Python | All Go, single binary on host |
| Docker hosts everything | Docker hosts LightRAG + WebUI only |
| Python MCP started from watchdog `start` | `coderag mcp` is the MCP server itself |

### Steps

| # | Step | What changes |
|---|------|-------------|
| 1 | Restructure `watchdog/internal/*` → `pkg/watchdog/` | Move watcher, diff, pipeline, parser, router, filter, crud to `pkg/watchdog/` with shared `go.mod` |
| 2 | Create `pkg/lightrag/` HTTP client | Extract from `pkg/watchdog/crud/` — shared between MCP and watchdog |
| 3 | Create `pkg/mcp/` with mark3labs/mcp-go | Go MCP server: 22 tools, stdio transport |
| 4 | Create `cmd/coderag/main.go` | CLI with `start/mcp/watch/status/stop/query/webui` subcommands |
| 5 | Wire `coderag mcp` → Docker + watcher + MCP | Auto-start Docker compose, wait for health, start watcher goroutine, enter stdio loop |
| 6 | Wire `coderag start` → Docker + watcher | Same Docker logic, foreground watcher, Ctrl+C shutdown |
| 7 | Wire `coderag webui` → open browser | Check Docker, print URL |
| 8 | Standalone `watchdog` entry point | Thin wrapper at `watchdog/cmd/watchdog/main.go` importing `pkg/watchdog` |
| 9 | Remove `daniel-lightrag-mcp/` | All functionality replaced by `pkg/mcp/` |
| 10 | Move `analysers/` to root | From `daniel-lightrag-mcp/analysers/` to `analysers/` |
| 11 | Update go.mod, Dockerfile, docker-compose.yml | Single Go module, Docker image has LightRAG + WebUI only |
| 12 | Update .opencode/opencode.json | MCP points to `["coderag", "mcp", "."]` |

### Package Dependency

```
cmd/coderag/
├── pkg/mcp/          (mark3labs/mcp-go)
│   └── pkg/lightrag/ (HTTP client)
│       └── (net/http)
├── pkg/watchdog/
│   ├── watcher/      (fsnotify)
│   ├── diff/
│   ├── pipeline/
│   ├── parser/
│   ├── router/
│   ├── filter/
│   └── crud/ → pkg/lightrag/ (or replaced by it)
│
watchdog/cmd/watchdog/
  └── pkg/watchdog/
```

### Key CLI behavior

```
coderag mcp .
  → docker compose up -d (if LightRAG not healthy)
  → wait for :9621
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
```

### Docker + Podman

- `coderag mcp` and `coderag start` first try Docker, fall back to Podman
- Detection: `docker compose` → `podman-compose` → `podman`
- No direct subprocess fallback — containers are required for LightRAG + WebUI
- WebUI static files built into the Docker image, served by LightRAG API

---

## Future Phases

### Phase 22 — Python Analyser
- Add Python analyser (AST-based)
- Register in `pkg/watchdog/router/file.go`
- Entity types: CLASS, FUNCTION, IMPORT, DECORATOR

### Phase 23 — Git Commit Watcher
- Extend git watcher to detect new commits (not just uncommitted changes)
- Support `git pull` → re-scan changed files

### Phase 24 — Stale Entity Cleanup
- Periodic sweep: entities whose `file_path` no longer exists on disk
- Configurable interval, dry-run mode

### Phase 25 — WebUI Integration
- WebUI served from LightRAG API (static files)
- Browse entity graph, search, query
- Visualize file → entity → relation mapping

---

## Security

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
