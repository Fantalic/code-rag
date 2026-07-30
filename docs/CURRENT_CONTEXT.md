# Current Context

## Status
- 🔥 **Phase 35** — Batch Import Performance Fix ✅ COMPLETE
- 🔥 **Phase 36** — Deploy-Ready CLI 🔧 IN PROGRESS
- **Current session (July 30, 2026)** — Phase 36.1: CLI Polish & UX

### Recent Achievements (July 30, 2026)

| Achievement | Detail | Status |
|-------------|--------|--------|
| Batch import API | `abatch_import()` with single lock, batch VDB upsert (`asyncio.gather`), single save, timing logs | ✅ Done |
| `POST /graph/batch-import` | New endpoint with `BatchImportRequest` models | ✅ Done |
| Go `BatchImport` client | `BatchImportPayload` + `BatchImport()` in `crud/client.go` | ✅ Done |
| Scan/watcher integration | Both `applyScanResults` and `applyCSharpResults` use `BatchImport` | ✅ Done |
| Verified performance | 1701 entities + 1875 relations in **59.25s** (~30x speedup) | ✅ Verified |

### Previous phases completed

| Phase | What | Status |
|-------|------|--------|
| 21 | coderag Restructuring (Go CLI, Docker, MCP bridge) | ✅ Done |
| 22 | MCP Tool Improvements (13 tools) | ✅ Done |
| 23-24 | MCP Bug Fixes & Auto-Initialization | ✅ Done |
| 25 | Architecture Cleanup & MCP Fixes | ✅ Done |
| 26-27 | Fix Broken MCP Tools & Verify | ✅ Done |
| 27.5 | Graph Quality Fix (workspace targeting, error logging) | ✅ Done |
| 28 | Go Native C# Analyser | ✅ Done |
| 32 | Knowledge Graph Refactoring (qualified names, calls, namespaces) | ✅ Done |
| 33 | MCP Tool Bug Fixes (codebase_stats, related_files, add_note) | ✅ Done |
| 34 | Test on Larger Project (dotnet-console-games) | ✅ Done |
| 35 | Batch Import Performance Fix | ✅ Done |

### Architecture

```
Host side:                        Docker:
  coderag (Go CLI)                ├── lightrag (API + WebUI :9621)
    ├── pkg/watchdog/                └── mcp (Python MCP SSE :9622)
    │   ├── watcher/   (fsnotify + git)
    │   ├── diff/      (entity/relation diff)
    │   ├── pipeline/  (analyser -> diff -> filter -> CRUD)
    │   ├── parser/    (C# analyser output)
    │   ├── router/    (ext -> analyser mapping)
    │   ├── filter/    (PII/keys/secrets)
    │   └── crud/      (LightRAG HTTP client)
    └── pkg/lightrag/  (alternate HTTP client)
```

## Running System

### Services
| Service | Type | Port | Status |
|---------|------|------|--------|
| `lightrag` (API + WebUI) | Docker | 9621 | healthy |
| `mcp` (MCP SSE) | Docker | 9622 | running |

### coderag CLI
- `cmd/coderag/main.go` -- CLI entry point (963 lines)
- `go build ./cmd/coderag` succeeds, `go vet ./...` passes
- Module: `github.com/coderag`, Go 1.25.5

### Commands
| Command | Description |
|---------|-------------|
| `coderag start <dir>` | Start Docker + file watcher |
| `coderag watch <dir>` | File watcher only |
| `coderag scan <dir>` | Run C# analyser, populate graph |
| `coderag mcp <dir>` | MCP stdio-SSE bridge |
| `coderag status` | Check services |
| `coderag stop` | Docker compose down |
| `coderag query <txt>` | Query the graph |
| `coderag webui` | Open browser |

## Current Task: Phase 36.1 -- CLI Polish & UX

See `docs/TASK.md` for full breakdown.

### What's missing

| # | Issue | Location | Severity |
|---|-------|----------|----------|
| 1 | Ctrl+C doesn't work (`signal.Notify` never called) | `cmd/coderag/main.go:299,368` | Critical |
| 2 | No `--version` flag | `cli.App{}` | Medium |
| 3 | No `--quiet` / `--verbose` -- all output is raw `log.Printf` | All commands | Medium |
| 4 | No progress indicators for Docker waits / scans | `ensureLightRAG`, `ensureMCP`, `scanCmd` | High |
| 5 | `coderag status` uses `log.Printf` (timestamped lines) | `statusCmd` | Medium |
| 6 | Docker compose output spills to CLI stdout | `startCmd`, `scanCmd` | Low |
| 7 | `openBrowser` missing Linux/macOS support | `cmd/coderag/main.go:320` | Medium |
| 8 | Duplicate startup logic in `startCmd` / `watchCmd` | `cmd/coderag/main.go` | Low |

### Key Technical Decisions

1. **Docker-first** -- LightRAG + WebUI + MCP always in containers
2. **No Ollama in Docker** -- assumed on host at `http://localhost:11434`
3. **Python MCP kept in Docker** -- no Go rewrite, SSE transport on port 9622
4. **Go native analyser** -- no CGo/tree-sitter, fully static binary
5. **Entity-level CRUD over document pipeline** -- faster, no LLM calls
6. **Security filter in pipeline** -- secrets never reach graph
