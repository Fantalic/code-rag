# CodeGraph Agent Guidelines

**coderag** is the central CLI tool for the codeRAG system. It transforms codebases into knowledge graphs for AI agent consumption.

## Project Structure

```
code-rag/
├── cmd/
│   └── coderag/main.go          ← CLI entry point → builds `coderag` binary
├── pkg/
│   ├── mcp/                     ← MCP server (mark3labs/mcp-go, 22 tools)
│   ├── watchdog/                ← File watcher (importable Go package)
│   │   ├── watcher/             ← fsnotify + git observer
│   │   ├── diff/                ← Entity/relation diff engine
│   │   ├── pipeline/            ← Pipeline orchestration
│   │   ├── parser/              ← Analyser output parsers
│   │   ├── router/              ← File extension → analyser mapping
│   │   ├── filter/              ← Security filter (PII/keys/secrets)
│   │   └── crud/                ← LightRAG HTTP CRUD client
│   └── lightrag/                ← LightRAG HTTP API client
├── analysers/                   ← Language analysers
│   └── csharp/                  ← C# Go native analyser
├── LightRAG/                    ← Python RAG engine (Docker)
│   ├── lightrag/                ← Core Python package
│   ├── lightrag-api/            ← FastAPI service
│   └── lightrag_webui/          ← React 19 + TypeScript UI
├── .opencode/                   ← CodeGraph prompts + opencode.json
├── Dockerfile                   ← LightRAG + WebUI image
├── docker-compose.yml           ← LightRAG + optional Ollama
└── go.mod
```

## Quick Start

```powershell
# Build coderag CLI
go build -o coderag ./cmd/coderag

# Start everything (Docker + file watcher)
coderag start <project-dir>

# MCP server mode (for opencode)
coderag mcp <project-dir>
```

## Development Commands

```bash
go build -o coderag ./cmd/coderag         # Build CLI
coderag start <dir>                        # Docker + file watcher
coderag mcp .                              # MCP bridge for opencode
docker compose up -d                       # Start LightRAG + Ollama
go test ./...                              # Go tests
pytest tests/ -v                           # Python tests
go vet ./...                               # Lint Go
ruff check .                               # Lint Python
```

## Coding Style

### Go
- Standard Go formatting (`gofmt`)
- Error handling: return errors, log at caller
- Use `pkg/lightrag` client for all LightRAG API calls
- Use `pkg/watchdog` for file watching and diff logic
- goroutines for concurrent watchers, channels for events
- snake_case for filenames, PascalCase for exported identifiers

### Python
- PEP 8 with four-space indentation, line length 88 (Black)
- Type annotations on all functions
- snake_case for functions/variables, PascalCase for classes
- Imports grouped: stdlib → third-party → local (isort with Black profile)

### TypeScript/React
- Two-space indentation
- Functional components with hooks
- PascalCase.tsx for component files
- Tailwind utility styling

### C# (Analyser)
- .NET standard conventions
- PascalCase for types/methods, camelCase for locals

## Testing
- `go test ./...` for Go packages
- `pytest tests/ -v` for Python tests
- Export required `LIGHTRAG_*` env vars before integration tests

## Documentation

| File | What it's for | Keep updated |
|------|---------------|--------------|
| `docs/DOCUMENTATION.md` | High-level system reference — architecture, components, pipeline, security | ✅ **Always** |
| `docs/API_REFERENCE.md` | Detailed API schemas — every endpoint with request/response | ✅ **Always** |
| `docs/PLAN.md` | High-level roadmap — where we're going, what's next | ✅ Update when phases change |
| `docs/CURRENT_CONTEXT.md` | Session memory — current state, what's been done, key decisions | ✅ Update after each task |
| `docs/TASK.md` | Active task breakdown — detailed steps for what we're building now | ✅ Update before/after each task |

## Current Task
The active sub-task prompt is in `docs/TASK.md`. Read it first before starting any work on the current phase.

## CodeGraph Scanning
See `.opencode/` for detailed agent instructions on scanning codebases:
- `code-graph.md`: Knowledge graph entity/relationship model
- `scan-complete.md`: Full project scanning (6-task approach)
- `scan-targeted.md`: Targeted entry-point scanning (6-task approach)

## Git Workflow
- Work with git and make commits frequently — commit after each completed task
- Before committing: `git status`, `git diff` to verify only intended files are staged
- Write concise commit messages that match the repo style
- Never commit `.env`, secrets, or generated build artifacts

## Security
- Never commit `.env` or secrets
- Copy `.env.example` and `config.ini.example` for configuration
- Use `LIGHTRAG_*` environment variables for storage backends
