# CodeGraph Agent Guidelines

This is a **forked monorepo** based on LightRAG, extended with an MCP server, code analysers, and a file watchdog. The project transforms codebases into knowledge graphs for AI agent consumption.

## Project Structure

```
code-lightrag/
├── LightRAG/                    ← Main fork: RAG engine + API + WebUI
│   ├── lightrag/                ← Core Python package
│   ├── lightrag-api/            ← FastAPI service
│   └── lightrag_webui/          ← React 19 + TypeScript UI
├── daniel-lightrag-mcp/         ← MCP server (22 tools for agent access)
│   ├── src/daniel_lightrag_mcp/ ← Python MCP server code
│   └── analysers/               ← Code analysers (C#, etc.)
├── watchdog/                    ← File change watcher for auto-ingestion
│   └── main.py                  ← Watchdog script
├── .opencode/                   ← CodeGraph prompts (scan-complete, scan-targeted, code-graph)
└── .clinerules/                 ← Agent rules
```

## Python Environment

**Always use a venv.** Never install or run Python commands in the global/system interpreter.

```powershell
python -m venv venv
.\venv\Scripts\Activate.ps1
```

## Quick Start (one command)

```powershell
.\scripts\setup.ps1          # Full install: LightRAG[api] + MCP + watchdog
```

Or manually (inside venv):

```bash
pip install -e LightRAG\[api]
pip install -e daniel-lightrag-mcp\[dev]
pip install watchdog
```

## Development Commands

### LightRAG server
```bash
lightrag-server              # or uvicorn lightrag.api.lightrag_server:app --reload
```

### MCP server (connects to running LightRAG on :9621)
```bash
python -m daniel_lightrag_mcp
```

### Watchdog (auto-ingest files into LightRAG)
```bash
python watchdog/main.py
```

### Test (root suite, 63 tests)
```bash
pytest tests/ -v
pytest daniel-lightrag-mcp/tests/ -v  # MCP test suite (163 tests)
```

### Lint
```bash
ruff check .
```

## Coding Style

### Python
- PEP 8 with four-space indentation, line length 88 (Black)
- Type annotations on all functions
- Use `lightrag.utils.logger` instead of `print`
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
- Always activate the venv first: `.\venv\Scripts\Activate.ps1`
- pytest for Python (functions start with `test_`)
- Vitest for UI components
- Export required `LIGHTRAG_*` env vars before integration tests

## Project Docs
See `docs/` for project plan, context tracking, and documentation.

## Current Task
The active sub-task prompt is in `docs/TASK.md`. Read it first before starting any work on the current phase.

## CodeGraph Scanning
See `.opencode/` for detailed agent instructions on scanning codebases:
- `code-graph.md`: Knowledge graph entity/relationship model
- `scan-complete.md`: Full project scanning (6-task approach)
- `scan-targeted.md`: Targeted entry-point scanning (6-task approach)

## Git Workflow (inline with KEEPTRACK.md)
- Work with git and make commits frequently — commit after each completed task
- Follow KEEPTRACK.md steps 5-6-7: write before acting, update after acting, keep PLAN.md current
- Before committing: `git status`, `git diff` to verify only intended files are staged
- Write concise commit messages that match the repo style
- Never commit `.env`, secrets, or generated build artifacts

## Security
- Never commit `.env` or secrets
- Copy `.env.example` and `config.ini.example` for configuration
- Use `LIGHTRAG_*` environment variables for storage backends
