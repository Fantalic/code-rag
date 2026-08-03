# CodeGraph Agent Guidelines

**coderag** is the central CLI tool for the codeRAG system. It transforms codebases into knowledge graphs for AI agent consumption.

## Project Structure

```
code-rag/
├── coderag/                      ← All owned code
│   ├── cli/                      ← Go CLI (orchestrator)
│   │   ├── main.go               ← Entry point
│   │   ├── go.mod
│   │   ├── watcher/              ← File watcher
│   │   ├── diff/                 ← Entity/relation diff engine
│   │   ├── pipeline/             ← Pipeline orchestration
│   │   ├── parser/               ← Analyser output parser
│   │   ├── router/               ← File extension → analyser mapping
│   │   ├── filter/               ← Security filter (PII/keys/secrets)
│   │   ├── crud/                 ← LightRAG HTTP CRUD client
│   │   └── analysers/csharp/     ← C# Go native analyser
│   ├── lightrag/                 ← Python RAG engine (Docker)
│   │   ├── lightrag/             ← Core Python package
│   │   ├── lightrag-api/         ← FastAPI service
│   │   └── lightrag_webui/       ← React 19 + TypeScript UI
│   ├── mcp/                      ← Python MCP server (Docker)
│   └── tests/                    ← Integration tests
├── scripts/                      ← Setup & build scripts
├── docker-compose.yml
├── Dockerfile
└── _dev_docs/                    ← Private working docs (gitignored)
```

## Quick Start

```bash
# Build coderag CLI
go build -C coderag/cli -o coderag .

# Start everything (Docker + file watcher)
coderag start <project-dir>

# MCP server mode (for opencode)
coderag mcp <project-dir>
```

## Development Commands

```bash
# Build CLI
go build -C coderag/cli -o coderag .

# Build C# analyser
cd coderag/cli/analysers/csharp && go build -o csharp_analyser.exe ./cmd/csharp_analyser/ && cd ../../..

# Deploy C# analyser (Windows)
copy coderag\cli\analysers\csharp\csharp_analyser.exe _dev_docs\analysers\csharp\

# Docker + file watcher
coderag start <dir>

# Run analyser
coderag scan <dir>

# MCP bridge
coderag mcp .

# Start Docker services
docker compose up -d

# Go tests
go test ./...

# Python tests
pytest tests/ -v

# Lint Go
go vet ./...

# Lint Python
ruff check .
```

## Coding Style

### Go
- Standard Go formatting (`gofmt`)
- Error handling: return errors, log at caller
- Use `crud` package for all LightRAG API calls
- Use `watchdog/*` packages for file watching and diff logic
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
| `DOCUMENTATION.md` | High-level system reference — architecture, components, pipeline, security | ✅ **Always** |
| `_dev_docs/ARCHITECTURE.md` | Architecture overview | ✅ **Always** |
| `_dev_docs/TASK.md` | Active task breakdown | ✅ Update before/after each task |
| `_dev_docs/CURRENT_CONTEXT.md` | Session memory | ✅ Update after each task |

## Current Task
The active sub-task prompt is in `_dev_docs/TASK.md`. Read it first before starting any work on the current phase.

## CodeGraph Scanning
See `_dev_docs/` for detailed agent instructions on scanning codebases:
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
- Copy `.env.example` for configuration
- Use `LIGHTRAG_*` environment variables for storage backends
