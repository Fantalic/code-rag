# code-lightrag Monorepo Restructure Plan (COMPLETED)

## Project Vision
**CodeGraph** — a codebase analysis system that:
1. Uses a **C# analyzer** (`daniel-lightrag-mcp/analysers/`) to extract classes, methods, relations from source code
2. **LightRAG** (heavily forked) stores everything as a knowledge graph for semantic search
3. **MCP server** (`daniel-lightrag-mcp/`) exposes 22+ tools so AI agents can query the codebase
4. **Watchdog** utility (`watchdog/`) auto-ingests changed files

## Current State (achieved)
```
code-lightrag/                    ← SINGLE git repo (main fork)
├── .git/
├── .opencode/                    ← project-wide prompts
├── .clinerules/                  ← project-wide rules
├── AGENTS.md                     ← consolidated guidelines
├── LICENSE                       ← MIT
├── README.md                     ← with full attribution
├── PLAN.md                       ← this file
├── LightRAG/                     ← subtree (main fork, full history)
├── daniel-lightrag-mcp/          ← subtree (MCP server, full history)
└── watchdog/                     ← subtree (file watcher, full history)
```

## Completed Steps
1. ✅ Initialize parent repo (`git init`, `.gitignore`, initial commit)
2. ✅ Add LightRAG as subtree (from `request-handeling` branch)
3. ✅ Move `.opencode/`, `.clinerules/`, `AGENTS.md` to root
4. ✅ Add daniel-lightrag-mcp as subtree (from `own-fork` branch, with uncommitted changes applied)
5. ✅ Add watchdog as subtree (from `master` branch)
6. ✅ Create LICENSE, README.md, AGENTS.md
7. ⬜ Verification

## License Attribution
- **LightRAG** — MIT License, Copyright (c) 2025 LightRAG Team (HKUDS). Source: https://github.com/HKUDS/LightRAG
- **daniel-lightrag-mcp** — MIT License, Copyright (c) 2025 Daniel Simpkins. Source: https://github.com/desimpkins/daniel-lightrag-mcp
- **watchdog** — MIT License (user's own work)
- This fork's additional code — MIT License
