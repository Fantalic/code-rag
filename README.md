<p align="center">
  <img src="https://capsule-render.vercel.app/api?type=rect&color=0:1a1a2e,100:16213e&height=120&section=header&text=codeRAG&fontSize=60&fontColor=e94560&fontAlignY=55" alt="codeRAG"/>
</p>

<p align="center">
  <i>Real-time codebase knowledge graphs for C# — queried by AI agents via MCP.</i>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-experimental-e94560?style=flat-square" alt="experimental"/>
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/Docker-24+-2496ED?style=flat-square&logo=docker&logoColor=white" alt="Docker"/>
  <a href="https://github.com/Fantalic/code-rag/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-9b59b6?style=flat-square" alt="License"/></a>
</p>

---

## What this is

A proof-of-concept tool that turns a C# solution into a **live, queryable knowledge graph**.  
Instead of grepping files or reading static docs, AI agents ask the graph directly via MCP.

Built to explore what's possible when deterministic code analysis meets local LLMs — no cloud, no data leaving the machine.

---

## What it does

| Capability | How |
|---|---|
| **Index** | Native Go parser extracts entities & relations from `.sln` deterministically |
| **Sync** | Live file watcher (`fsnotify` + git polling) keeps the graph current |
| **Query** | 10 MCP tools for agents: existence checks, dependency maps, dead-code detection, circular-dependency analysis |
| **Explore** | Web UI at `localhost:9621` for visual graph navigation |
| **Answer** | Natural-language queries via local Ollama (optional) |

---

## Try it

```bash
git clone https://github.com/Fantalic/code-rag.git && cd code-rag
bash scripts/setup.sh        # Windows: .\scripts\setup.ps1

coderag start ./myproject    # spin up services + watcher
coderag scan  ./myproject    # index your .sln
coderag webui                # open graph explorer
coderag mcp   ./myproject    # bridge for your AI agent
coderag query "How does checkout work?"
```

Requirements: Go 1.25+, Docker 24+, Ollama 0.3+.  
Full setup & troubleshooting → [`docs/INSTALL.md`](docs/INSTALL.md)

---

## Architecture

```
C# solution (.sln)
      │
      ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ Go analyzer │────▶│ LightRAG    │◀────│ Go watcher  │
│ (determin.) │     │ (:9621)     │     │ (live sync) │
└─────────────┘     └──────┬──────┘     └─────────────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
         ┌────────┐   ┌────────┐   ┌────────┐
         │ Web UI │   │ MCP    │   │ Ollama │
         │        │   │ Server │   │ (host) │
         └────────┘   └────────┘   └────────┘
```

Deep dive → [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)

---

## Documentation

| File | Topic |
|---|---|
| [`docs/INSTALL.md`](docs/INSTALL.md) | Setup, models, offline mode, troubleshooting |
| [`docs/USAGE.md`](docs/USAGE.md) | Full CLI reference |
| [`docs/MCP_CLIENTS.md`](docs/MCP_CLIENTS.md) | Wiring into Claude, Cursor, OpenCode, Windsurf |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Components, data flow, design decisions |
| [`docs/SECURITY.md`](docs/SECURITY.md) | Redaction rules |

---

## Tech stack

- **Go** — CLI, C# parser, file watcher, MCP bridge
- **Python / LightRAG** — Knowledge graph engine
- **Docker** — Containerized MCP server + graph backend
- **Ollama** — Local LLM + embeddings

Built on [LightRAG](https://github.com/HKUDS/LightRAG) (MIT) and [MCP](https://modelcontextprotocol.io/) (MIT).

---

## Status

Experimental. Core indexing and MCP tooling work.  
Natural-language querying is functional but rough.  
Feedback welcome — open an issue or reach out.

---

<p align="center">
  <sub>MIT License © <a href="https://github.com/Fantalic">Fantalic</a></sub>
</p>
