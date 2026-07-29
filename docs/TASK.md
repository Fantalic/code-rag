# CRUD Pipeline Fix — Parallel Worker Race + Timeout

## Current Task

Fix the `coderag start` initial sync failure: 4 workers each process ALL 348 analyser results in parallel, causing massive "Entity already exists" errors and "context deadline exceeded" timeouts.

## What's Broken (Pre-Fix)

| Issue | Description | Fix |
|-------|-------------|-----|
| 4 workers process all results in parallel | `observer.go:initialSync()` enqueues each of 348 files with FULL results → 4 workers each call `applyCSharpResults` with all 1701 entities | ✅ Mutex serializes calls |
| No entity dedup across workers | `relationDedup` exists but no entity-level parallel protection | ✅ Mutex renders unnecessary |
| `DefaultTimeout=30s` too short | Per-entity create call hits 30s timeout when server loaded | ✅ Increased to 300s |
| `httpClient.Timeout=30` same issue | Pipeline's HTTP client had same short timeout | ✅ Increased to 300 |
| Storage in Docker volume, not `.coderag` | LightRAG stores graph in volume `rag_data`, not project `.coderag` | 📋 Pending |

## Plan

### ✅ Step 1: Add mutex to `applyCSharpResults`
- `pkg/watchdog/pipeline/run.go`: added `applyCSharpResultsMu sync.Mutex`
- Acquired with `Lock()` / `defer Unlock()` at function entry
- Worker 1 creates all entities; workers 2-4 see diff=empty, fast exit

### ✅ Step 2: Increase timeouts
- `pkg/watchdog/crud/client.go`: `DefaultTimeout 30s → 300s`
- `pkg/watchdog/pipeline/run.go`: `httpClient.Timeout 30 → 300`

### Step 3: Rebuild and verify
- ✅ `go build -o coderag.exe ./cmd/coderag` succeeds
- ✅ `go vet ./...` passes
- ✅ `go test ./pkg/watchdog/pipeline/` passes
- 📋 Run `coderag start` on `dotnet-console-games` and verify graph has entities AND relations

### Step 4: Fix storage location
- Map `rag_data` Docker volume to project `.coderag/rag_storage/` instead of Docker-managed volume
- Requires changes to `docker-compose.yml` (bind mount vs volume)

### Step 5: Document
- ✅ Update CURRENT_CONTEXT.md
- ✅ Update TASK.md