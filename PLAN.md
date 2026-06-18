# code-lightrag Monorepo Restructure Plan

## Project Vision
CodeGraph — a codebase analysis system that:
1. Uses a **C# analyzer** to extract classes, methods, relations from source code
2. **LightRAG** (heavily forked) stores everything as a knowledge graph for semantic search
3. **MCP server** exposes 22+ tools so AI agents can query the codebase instead of reading every file
4. **Watchdog** utility auto-ingests changed files

## Current State
```
code-lightrag/
├── LightRAG/                    ← git: forked from HKUDS/LightRAG (202 files changed)
│   ├── .git/
│   ├── .opencode/               ← tracked in LightRAG, but are project-wide prompts
│   ├── .clinerules/             ← tracked in LightRAG, but are project-wide rules
│   ├── lightrag/                ← core Python package
│   ├── lightrag-api/            ← FastAPI server
│   ├── lightrag_webui/          ← React UI
│   └── ...
├── daniel-lightrag-mcp/         ← git: forked from desimpkins/daniel-lightrag-mcp
│   ├── .git/
│   ├── analysers/               ← C# analyzer binaries
│   └── src/daniel_lightrag_mcp/ ← MCP server code
└── watchdog/                    ← git: user's own utility
    ├── .git/
    ├── watchdog.py
    └── rag_input/
```

## Target State
```
code-lightrag/                    ← SINGLE git repo (main fork)
├── .git/
├── .opencode/                    ← project-wide, moved from LightRAG/
├── .clinerules/                  ← project-wide, moved from LightRAG/
├── AGENTS.md                     ← consolidated from all sub-projects
├── LICENSE                       ← MIT
├── README.md                     ← with full attribution
├── PLAN.md                       ← this file
├── LightRAG/                     ← subtree (main fork, full history preserved)
├── daniel-lightrag-mcp/          ← subtree (MCP server, full history preserved)
└── watchdog/                     ← subtree (file watcher, full history preserved)
```

## Execution Steps

### Step 1 — Initialize parent repo
- `git init` in code-lightrag/
- Create `.gitignore` for the parent

### Step 2 — Add LightRAG as subtree
- `git subtree add --prefix=LightRAG ./LightRAG/ request-handeling`
- This preserves all of LightRAG's commit history as part of the parent repo

### Step 3 — Move project-wide config to root
- Move `LightRAG/.opencode/` → `./.opencode/`
- Move `LightRAG/.clinerules/` → `./.clinerules/`
- Remove originals from `LightRAG/`
- Commit

### Step 4 — Add daniel-lightrag-mcp as subtree
- `git subtree add --prefix=daniel-lightrag-mcp ./daniel-lightrag-mcp/ own-fork`
- Preserves all MCP server commit history

### Step 5 — Add watchdog as subtree
- `git subtree add --prefix=watchdog ./watchdog/ feat/git-watcher`
- Preserves all watchdog commit history

### Step 6 — Create root-level files
- `LICENSE` — MIT, with proper copyright attribution
- `README.md` — project overview, architecture, license attribution
- `AGENTS.md` — consolidated coding guidelines from all sub-projects

### Step 7 — Verification
- `git log --oneline` to verify all histories are present
- Check that all sub-projects are at expected paths
- Verify no broken imports or references

## License Attribution
- **LightRAG** — MIT License, Copyright (c) 2025 LightRAG Team (HKUDS). Source: https://github.com/HKUDS/LightRAG
- **daniel-lightrag-mcp** — MIT License, Copyright (c) 2025 Daniel Simpkins. Source: https://github.com/desimpkins/daniel-lightrag-mcp
- **watchdog** — MIT License (user's own work)
- This fork's additional code — MIT License, Copyright (c) 2025
