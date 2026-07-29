# Cascade-Aware Deletion + Incremental Initial Sync

## Current Task

Fix `coderag start` to use git-based incremental approach: detect only changed/deleted files, process only what's needed, cascade deletions to affected files, and ensure relation `source_id` is correctly stored in LightRAG.

## What's Broken

| Issue | Description | Status |
|-------|-------------|--------|
| `initialSync()` processes everything | Full analyser run on ALL solutions, queued ALL files — wasteful on repeated `coderag start` | ✅ Fixed (git diff filtering) |
| Deleted files silently dropped | `getGitChangedFiles` filtered against filesystem walk — deletions never detected | ✅ Fixed (`getGitDeletedFiles`) |
| No cascade on deletion | Deleting a file left stale relations and affected files out of sync | ✅ Fixed (`queueDeletionWithCascade`) |
| `findComposeFile()` fails outside repo | Only searched cwd and `../` — not relative to binary | ✅ Fixed (adds binary dir) |
| `Relation` parser missing `SourceID`/`FilePath` | Struct has no fields — relations get wrong `source_id` in LightRAG | ✅ Fixed |
| `findAffectedFiles` uses broken `rel.SourceID` | All relations have `source_id` = trigger file — cascade targets wrong files | ✅ Fixed (entity-based fallback) |
| Vector DB consistency after cascade | Entity deletion removes graph nodes, edges, and VDB embeddings | ✅ Verified (integration test) |
| `parserRelationsToMapScan` drops `source_id`/`file_path` | `coderag scan` relations lose `source_id`/`file_path` in LightRAG — all 43 relations stored with empty values | ✅ Fixed |
| `loadCurrentRelations`/`loadCurrentRelationsScan` don't read `source_id`/`file_path` | Diff engine can't detect changes to these fields on existing relations | ✅ Fixed |
| CALL relations missing `source_id`/`file_path` | `parseCallLine` doesn't read caller file path, `ParseCSharpFile` doesn't set `SourceID` for CALL lines | ✅ Fixed |
| Stale analyser output files persist between scans | `RunCSharpAnalyser` reads ALL `.txt` files including from deleted sources | ✅ Fixed (cleanup before run) |

## Completed

### Observer changes (`pkg/watchdog/watcher/observer.go`)
- `initialSync()` — git-based: build hash map → get changed files → get deleted files → queue deletions with cascade → process changes per-solution
- `getGitChangedFiles(files)` — returns files that exist on disk AND differ from git HEAD
- `getGitDeletedFiles()` — returns files in git HEAD but removed from disk
- `queueDeletionWithCascade(delPath)` — remove from hash map, query entities, find affected files via relations, queue re-scan for affected files, queue "deleted" event
- `findAffectedFiles(entityNames, excludePath)` + `findAffectedFilesFromData()` — HTTP layer + pure logic, with entity-map fallback for wrong `source_id`
- `checkGitChanges()` + `handleEvent()` — cascade on deletion via `queueDeletionWithCascade`

### CRUD client (`pkg/watchdog/crud/client.go`)
- `RelationEdge` struct — `{Source, Target, Keywords, SourceID, FilePath}`
- `GetAllRelations()` queries `/graphs?...` and returns `[]RelationEdge`

### `cmd/coderag/main.go`
- `inputDirOrDefault` — uses `filepath.Abs` to resolve `.` to absolute path
- `findComposeFile` — also searches relative to binary's own directory

### Relation parser + pipeline (`pkg/watchdog/parser/csharp.go`, `pkg/watchdog/pipeline/run.go`)
- Added `SourceID` and `FilePath` to `Relation` struct (matching `Entity` struct)
- `parseRelationLine` reads `parts[5]` (the file path from analyser output)
- `ParseCSharpFile` sets `rel.SourceID = filename` for both RELATION and CALL lines
- `parseCallLine` now reads caller file path from `parts[1]` (CallerFile field)
- `parserRelationsToMap` includes `"source_id"` and `"file_path"` in the data map

### `parserRelationsToMapScan` in `cmd/coderag/main.go` (NEW FIX)
- Was missing `source_id`/`file_path` fields — all relations created via `coderag scan` lost these fields in LightRAG
- Now mirrors `parserRelationsToMap`: includes `source_id`/`file_path` when non-empty

### `loadCurrentRelationsScan` in `cmd/coderag/main.go` (NEW FIX)
- Was only reading `keywords` from graph edge properties
- Now also reads `source_id` and `file_path` — diff engine can detect changes to these fields

### `loadCurrentRelations` in `pkg/watchdog/pipeline/run.go` (NEW FIX)
- Same fix as `loadCurrentRelationsScan` — reads `source_id`/`file_path` from graph edges

### `RunCSharpAnalyser` in `pkg/watchdog/pipeline/run.go` (NEW FIX)
- When `outputDir` is provided, cleans stale `.txt` files before running analyser
- Prevents deleted source files' output from being re-processed in subsequent scans

### Tests (all pass)
- Parser test updated for CALL relations now having `FilePath` (from caller file)

### Next: Implement 4 New MCP Tools

**Phase 32 — New MCP Tools**

| # | Proposed Tool | Purpose | Priority |
|---|--------------|---------|----------|
| 1 | `get_relation_detail(entity, relation_type)` | Show all relations of a specific type for an entity | High |
| 2 | `get_method_signature(entity_name)` | Show method signature (parameters, return type) | High |
| 3 | `find_related_files(entity_or_file)` | Find all files referencing/related to an entity | High |
| 4 | `get_interface_methods(interface_name)` | Show interface methods + implementing classes | High |

These address gaps identified during the Calculator implementation planning session. Each tool should be implemented in both the Python MCP server (`daniel-lightrag-mcp/`) and the Go MCP server, with tests and docs.
