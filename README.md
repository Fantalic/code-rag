<div align="center">

# 🧠 codeRAG

### *Your codebase, visualized. Your knowledge graph, alive.*

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![Python](https://img.shields.io/badge/Python-3.11+-3776AB?logo=python)](https://python.org)
[![MCP](https://img.shields.io/badge/MCP-10%20tools-blueviolet)](https://modelcontextprotocol.io)
[![Docker](https://img.shields.io/badge/Docker-required-2496ED?logo=docker)](https://docker.com)
[![Ollama](https://img.shields.io/badge/Ollama-required-black)](https://ollama.com)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Your C# solution as a live, queryable knowledge graph.**

Deterministic code indexing · Built-in dead-code analysis · Live file watcher ·
Circular-dependency detection · MCP tools for AI agents · 100% local by default

</div>

---

## Requirements

| Tool | Min version | Notes |
|------|-------------|-------|
| Go | 1.25+ | Only to build the CLI; not needed at runtime |
| Docker | 24+ | Runs LightRAG + MCP server |
| Ollama | 0.3+ | Local LLM + embeddings (host, not Docker) |
| OS | Linux, macOS, Windows 10/11 (WSL2 or Git Bash) | |

Models — pulled by the setup script, switchable via env vars (`EMBEDDING_MODEL`,
`LLM_MODEL`). By default the script pulls only the embedding model (`bge-m3:latest`);
the MCP tools need no LLM. Set `LLM_MODEL` (e.g. `phi3.5:latest`, or the
`qwen3-coder:480b-cloud` cloud model) before setup if you also want `coderag query`.

## Get started

```bash
git clone https://github.com/Fantalic/code-rag.git && cd code-rag
bash scripts/setup.sh        # Windows: .\scripts\setup.ps1

coderag start ./myproject    # start services + live watcher
coderag scan  ./myproject    # index a C# solution (needs a .sln)
coderag graph files          # per-file stats
coderag webui                # visual explorer at http://localhost:9621
coderag mcp   ./myproject    # bridge for your AI agent (opencode, Claude, Cursor, …)
coderag query "How does checkout work?"   # natural-language answer (LLM)
```

Manual install, offline mode, and troubleshooting → **[docs/INSTALL.md](docs/INSTALL.md)**
· full command reference → **[docs/USAGE.md](docs/USAGE.md)**

## MCP tools (10)

| Intent | Tool |
|---|---|
| "Does X exist?" | `exists_in_codebase` |
| "Find entities matching X" | `get_nodes` (entity_type, name, file_path, connected_to) |
| "Tell me about X" | `node_details` (details / relations / neighbors) |
| "Orientation" | `codebase_overview` |
| "What touches this file?" | `file_dependencies` |
| "Any cycles?" | `detect_circular_dependencies` |
| "What's dead?" | `dead_code` (min_age_days, entity_type) |
| "Structured retrieval" | `query_codebase` |
| "Leave / read notes" | `attach_note_to_node`, `get_notes` |

Agent wiring (opencode, Claude Desktop, Cursor, Windsurf) → **[docs/MCP_CLIENTS.md](docs/MCP_CLIENTS.md)**

## How it works (short)

1. **C# → graph.** A native Go analyser parses your `.sln`, extracts entities
   + relations deterministically, and pushes them to LightRAG via one batched import.
2. **Prose → optional LLM.** Non-code files can go through LightRAG's pipeline when you
   feed them explicitly.
3. **Live watcher.** fsnotify + a 30 s git poll keep the graph in sync; deletions cascade.
4. **MCP bridge.** `coderag mcp <dir>` converts your agent's stdio MCP to SSE and talks
   to the Python MCP server in Docker, which fronts the LightRAG graph.

```
AI agent (stdio) → coderag mcp (Go bridge) → MCP server (Docker :9622) → LightRAG (Docker :9621)
```

Ollama runs on the host; the C# analyser and the watcher are plain host processes.
Architecture deep-dive →
**[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**

## Documentation

| Doc | What it covers |
|-----|----------------|
| [docs/INSTALL.md](docs/INSTALL.md) | Manual install, model setup, offline mode, troubleshooting |
| [docs/MCP_CLIENTS.md](docs/MCP_CLIENTS.md) | Wire codeRAG into opencode, Claude Desktop, Cursor, Windsurf |
| [docs/VS.md](docs/VS.md) | Visual Studio / .NET developers |
| [docs/USAGE.md](docs/USAGE.md) | Full CLI reference, all flags, day-to-day workflow |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Components, data flow, host/Docker split |
| [docs/SECURITY.md](docs/SECURITY.md) | What gets redacted before reaching the graph |
| [docs/SUPPORTED_FILES.md](docs/SUPPORTED_FILES.md) | File types, analysers, ingestion paths |

A more guided tour, concepts, and configuration → **[DOCUMENTATION.md](DOCUMENTATION.md)**

## Status

**Status:** `experimental` 🧪 · **License:** MIT

Built on [LightRAG](https://github.com/HKUDS/LightRAG) (MIT) and
[modelcontextprotocol](https://github.com/modelcontextprotocol/python-sdk) (MIT).
