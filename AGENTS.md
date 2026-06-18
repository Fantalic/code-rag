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

## Build, Test, and Development Commands

### LightRAG core
- `pip install -e .` / `pip install -e .[api]`: install package
- `lightrag-server` or `uvicorn lightrag.api.lightrag_server:app --reload`: start API
- `ruff check .`: lint Python sources
- `python -m pytest tests`: run tests

### MCP Server
- `pip install -e ".[dev]"`: install in dev mode
- `pytest tests/`: run all tests
- `ruff check src/`: lint
- `python -m daniel_lightrag_mcp`: start MCP server

### Watchdog
- `python main.py`: run the file watcher

### Web UI
- `cd lightrag_webui && bun install && bun run dev`: start UI dev server

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
- pytest for Python (functions start with `test_`)
- Vitest for UI components
- Export required `LIGHTRAG_*` env vars before integration tests

## CodeGraph Scanning
See `.opencode/` for detailed agent instructions on scanning codebases:
- `code-graph.md`: Knowledge graph entity/relationship model
- `scan-complete.md`: Full project scanning (6-task approach)
- `scan-targeted.md`: Targeted entry-point scanning (6-task approach)

## Security
- Never commit `.env` or secrets
- Copy `.env.example` and `config.ini.example` for configuration
- Use `LIGHTRAG_*` environment variables for storage backends
