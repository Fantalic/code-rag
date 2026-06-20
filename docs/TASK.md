# Phase 21 — coderag Restructuring

## Goal

Restructure the monorepo so `coderag` is the central CLI tool. Everything in modular Go packages. Python MCP server replaced by Go MCP built into the `coderag` binary. Docker/Podman for LightRAG + WebUI services.

## Steps

### Step 1 — Move `watchdog/internal/*` → `pkg/watchdog/`

Move all packages from `watchdog/internal/` to `pkg/watchdog/`:
- `crud/` → `pkg/watchdog/crud/` (will later be replaced by `pkg/lightrag/`)
- `diff/` → `pkg/watchdog/diff/`
- `filter/` → `pkg/watchdog/filter/`
- `parser/` → `pkg/watchdog/parser/`
- `pipeline/` → `pkg/watchdog/pipeline/`
- `router/` → `pkg/watchdog/router/`
- `watcher/` → `pkg/watchdog/watcher/`

Update all import paths from `github.com/code-lightrag/watchdog/internal/...` to `github.com/code-lightrag/coderag/pkg/watchdog/...`.

### Step 2 — Create `pkg/lightrag/` HTTP client

Extract the HTTP client from `pkg/watchdog/crud/` into a shared `pkg/lightrag/` package. This will be the canonical client for talking to the LightRAG API, used by both `pkg/mcp/` and `pkg/watchdog/`.

Files:
- `pkg/lightrag/client.go` — `Client` struct, `New()`, `Do()`, headers
- `pkg/lightrag/types.go` — shared request/response types
- `pkg/lightrag/entities.go` — entity CRUD methods
- `pkg/lightrag/relations.go` — relation CRUD methods
- `pkg/lightrag/documents.go` — document methods
- `pkg/lightrag/query.go` — query methods
- `pkg/lightrag/system.go` — health, pipeline status

### Step 3 — Create `pkg/mcp/` with mark3labs/mcp-go

Build the Go MCP server. Uses `github.com/mark3labs/mcp-go` for protocol handling.

Files:
- `pkg/mcp/server.go` — MCP server setup, stdio transport, init/tools/list/tools/call dispatch
- `pkg/mcp/tools.go` — all 22 tool registrations + handlers
  - 7 document management tools
  - 2 query tools
  - 9 knowledge graph tools
  - 4 system tools

Each handler:
- Calls `pkg/lightrag/` HTTP client for API tools
- Calls `pkg/watchdog/` directly for internal tools (e.g., `update_relations`)

### Step 4 — Create `cmd/coderag/main.go`

CLI entry point with subcommands:
- `start` — start Docker + file watcher
- `mcp` — start Docker + file watcher + MCP stdio server
- `webui` — open browser to WebUI
- `watch` — file watcher only
- `status` — health check
- `stop` — stop services
- `query` — quick KG query

Uses `github.com/urfave/cli/v2` (same as current watchdog).

### Step 5 — Wire `coderag mcp`

When `coderag mcp <dir>` is called:
1. Check LightRAG health at `http://localhost:9621/health`
2. If not healthy:
   a. Detect container runtime: Docker → `docker compose`, Podman → `podman-compose`
   b. Run `docker compose -f docker-compose.yml up -d`
   c. Wait for health (poll every 2s, timeout 60s)
3. Start file watcher on `<dir>` (goroutine)
4. Print "LightRAG ready at http://localhost:9621"
5. Enter MCP stdio loop (block until stdin closes)

### Step 6 — Wire `coderag start`

When `coderag start <dir>` is called:
1. Same Docker health check as Step 5
2. Start file watcher in foreground
3. Print status + WebUI URL
4. Block until Ctrl+C

### Step 7 — Wire `coderag webui`

When `coderag webui` is called:
1. Check LightRAG health
2. If healthy → open `http://localhost:9621` in default browser
3. If not healthy → print "LightRAG not running. Run 'coderag start <dir>' first."

### Step 8 — Standalone `watchdog` entry point

Thin wrapper at `watchdog/cmd/watchdog/main.go`:
```go
package main

import (
    "github.com/code-lightrag/coderag/pkg/watchdog/watcher"
)

func main() {
    // Same as current but imports from pkg/watchdog/
}
```

### Step 9 — Remove `daniel-lightrag-mcp/`

Delete `daniel-lightrag-mcp/` directory. All functionality replaced by `pkg/mcp/`.

### Step 10 — Move `analysers/` to root

Move `daniel-lightrag-mcp/analysers/` → `analysers/` at project root.
Move `daniel-lightrag-mcp/tests/` → `tests/` (merge with root tests).

### Step 11 — Update go.mod, Dockerfile, docker-compose.yml

**go.mod**: Update module path to `github.com/code-lightrag/coderag`, add `mark3labs/mcp-go` dependency.

**Dockerfile**: Multi-stage build:
- Stage 1: Build WebUI (node/bun)
- Stage 2: Python + LightRAG + WebUI static files

**docker-compose.yml**: LightRAG API + optional Ollama. No Go services.

### Step 12 — Update .opencode/opencode.json

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "coderag": {
      "type": "local",
      "command": ["coderag", "mcp", "."],
      "enabled": true,
      "env": {
        "LIGHTRAG_URL": "http://localhost:9621",
        "OLLAMA_HOST": "http://localhost:11434"
      }
    }
  }
}
```

## Verification

1. `go build ./cmd/coderag` — compiles `coderag` binary
2. `coderag mcp .` in test project — starts Docker, watcher, MCP loop
3. opencode connects to MCP server — 22 tools available
4. File change in project → watcher detects → diff → graph update
5. `coderag query "what classes?"` returns entities
6. `coderag webui` opens browser
7. `coderag status` shows healthy
8. `coderag stop` shuts down containers

## Files to create

| File | Purpose |
|------|---------|
| `cmd/coderag/main.go` | CLI entry |
| `pkg/mcp/server.go` | MCP server setup |
| `pkg/mcp/tools.go` | 22 tool handlers |
| `pkg/lightrag/client.go` | LightRAG HTTP client |
| `pkg/lightrag/types.go` | Shared types |
| `pkg/lightrag/entities.go` | Entity CRUD |
| `pkg/lightrag/relations.go` | Relation CRUD |
| `pkg/lightrag/documents.go` | Document methods |
| `pkg/lightrag/query.go` | Query methods |
| `pkg/lightrag/system.go` | Health, status |

## Files to modify

| File | Change |
|------|--------|
| `watchdog/internal/*` → `pkg/watchdog/*` | Move packages |
| `go.mod` | Module path → `github.com/code-lightrag/coderag` |
| `Dockerfile` | Multi-stage: WebUI build + LightRAG |
| `docker-compose.yml` | Remove Go services |
| `.opencode/opencode.json` | MCP → `["coderag", "mcp", "."]` |

## Files to delete

| File | Reason |
|------|--------|
| `daniel-lightrag-mcp/` | Replaced by `pkg/mcp/` |
| `watchdog/internal/` | Moved to `pkg/watchdog/` |
| `watchdog/go.mod` | Merged into root go.mod |
