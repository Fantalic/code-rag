# CodeGraph — Codebase Knowledge Graph System

**CodeGraph** transforms codebases into knowledge graphs for AI agent consumption. It uses a graph-based RAG engine to store classes, methods, and their relationships as a semantic knowledge graph, so agents can search and understand codebases without reading every source file.

## Architecture

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  C# Analyzer │───▶│   LightRAG   │◀───│   Watchdog   │
│  (extracts   │    │  (knowledge  │    │  (auto-      │
│   relations) │    │   graph DB)  │    │   ingests)   │
└──────────────┘    └──────┬───────┘    └──────────────┘
                           │
                    ┌──────▼───────┐
                    │  MCP Server  │
                    │  (22 tools   │
                    │   for agents)│
                    └──────────────┘
```

- **LightRAG** — Forked RAG engine that stores code entities and relations in a graph database for semantic search
- **C# Analyzer** (`daniel-lightrag-mcp/analysers/`) — Extracts classes, methods, inheritance, and dependency relations from C# codebases
- **MCP Server** (`daniel-lightrag-mcp/`) — 22 Model Context Protocol tools for AI agents to query the codebase knowledge graph
- **Watchdog** (`watchdog/`) — File system watcher that auto-ingests changed files into LightRAG

## Getting Started

```bash
# LightRAG core
cd LightRAG
pip install -e .
# or with API: pip install -e .[api]

# MCP server
cd ../daniel-lightrag-mcp
pip install -e ".[dev]"

# Watchdog
cd ../watchdog
pip install watchdog
```

## License & Attribution

This project incorporates code from several sources:

### LightRAG (MIT)
- **Source**: [https://github.com/HKUDS/LightRAG](https://github.com/HKUDS/LightRAG)
- **Copyright**: (c) 2025 LightRAG Team (HKUDS)
- **License**: MIT — see [LightRAG/LICENSE](LightRAG/LICENSE)
- **Changes**: Heavily forked with modifications to core, API, web UI, and storage backends

### daniel-lightrag-mcp (MIT)
- **Source**: [https://github.com/desimpkins/daniel-lightrag-mcp](https://github.com/desimpkins/daniel-lightrag-mcp)
- **Copyright**: (c) 2025 Daniel Simpkins
- **License**: MIT — see [daniel-lightrag-mcp/LICENSE](daniel-lightrag-mcp/LICENSE)

### watchdog
- User's own work
- **License**: MIT

### Additional Code
All new code, analysers, agent prompts, and modifications in this fork:
- **License**: MIT (see root [LICENSE](LICENSE))

## Project Structure

```
code-lightrag/
├── LightRAG/                    ← Forked RAG engine (main project)
├── daniel-lightrag-mcp/         ← MCP server + code analysers
├── watchdog/                    ← File change watcher
├── .opencode/                   ← CodeGraph agent scanning prompts
├── .clinerules/                 ← Agent rules
├── AGENTS.md                    ← Development guidelines
└── LICENSE                      ← MIT license
```
