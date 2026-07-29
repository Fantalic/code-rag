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

### Next: Implement 4 New MCP Tools (Phase 32) ✅ DONE

All 4 proposed MCP tools have been implemented in `daniel-lightrag-mcp/`:
- `get_relation_detail(entity_name, relation_type)` — filter edges by keywords
- `get_method_signature(entity_name)` — parse description for params/return type
- `find_related_files(entity_or_file)` — match by file_path or entity name
- `get_interface_methods(interface_name)` — find interface methods + implementors

---

## Phase 33 — MCP Tool Bug Fixes 🔧 IN PROGRESS

### Goal
Fix the broken MCP tools identified during testing so all 17 tools work reliably.

### Issues Found

| # | Bug | Affected Tool | Severity | Fix |
|---|-----|--------------|----------|-----|
| 1 | POST `/query` and `/query/data` time out — Ollama unreachable from Docker | `query`, `query_data`, `get_related_files`, `find_related_files` | Critical | Fix Ollama connectivity in docker-compose (host.docker.internal may not resolve) |
| 2 | `get_codebase_stats` returns HTTP 500 (`.split()` on list) | `get_codebase_stats` | High | Fix error handling in `client.py:get_codebase_stats` |
| 3 | `list_entities_by_type` server-side filter always returns empty | `list_entities_by_type` | High | Fix entity_type comparison in LightRAG client/server |
| 4 | Windows backslash path mismatch in `get_entities_in_file` | `get_entities_in_file` | Medium | Normalize paths (backslash → forward slash) in client |
| 5 | `add_note_to_entity` reports failure despite creating note | `add_note_to_entity` | Medium | Fix success status reporting in `_handle_add_note` |
| 6 | No snippet content in `get_related_files` response | `get_related_files` | Low | Fetch snippet content from chunks or document storage |
| 7 | Sub-agent connectivity documentation missing | All tools | Medium | Add tool-to-API endpoint mapping to docs |

### MCP Tool Fixes Applied (August 2026) ✅

| # | Bug | Tool | Fix | Status |
|---|-----|------|-----|--------|
| 1 | `keywords.split(",")` fails when keywords is a list | `get_codebase_stats` | Added `isinstance(keywords, list)` check before splitting | ✅ Fixed |
| 2 | `file_path` extracted from wrong location in query_data response | `get_related_files` | Extract from top-level `entity.get("file_path")` first, then `properties` fallback | ✅ Fixed |
| 3 | `add_note_to_entity` checks `created` flag (always False for notes) | `add_note_to_entity` | Changed to check `create_resp.status == "success"` | ✅ Fixed |

### Verification Results

All fixes verified against live test-csharp-project LightRAG API:
- `get_related_files` → 7 unique files found (was returning 0)
- `get_codebase_stats` → No more HTTP 500 errors
- `add_note_to_entity` → Returns `status: "success"` correctly (was returning error)
- `list_entities_by_type` → 7 CLASS + 29 METHOD entities (client-side filter working)
- `get_entities_in_file` → 8 entities in Calculator.cs (path normalization working)

---

## Phase 34 — Test on Larger Project (dotnet-console-games) ✅ DONE

**Goal:** Validate coderag MCP tools on a larger, more diverse C# codebase (50+ console games).

**Project:** `E:\_dev\_Projects\dotnet-console-games`
- Scanned: 55 projects, 348 files, 452 entities, 587 relations
- Solution file: `dotnet-console-games.sln`
- Graph health: All entity types (CLASS, METHOD, PROPERTY, ENUM, STRUCT) and relation types (defines, depends_on, extends, has, implements, invokes, references) populated

**Sub-agent prompt:** `docs/subagent-prompts/dotnet-console-games-explorer.md` (updated with correct HTTP API endpoints)

**Sub-agent exploration result:** The sub-agent successfully explored the graph, identified Hangman as a modification candidate, and proposed score tracking + difficulty levels. The sub-agent did not persist code changes (this was its intended scope — exploration and proposal).

**MCP tool evaluation:** See `docs/MCP_TOOL_EVALUATION.md` for the full test report. Key findings:
- Graph exploration tools work well (entity existence, graph dump, entity filtering)
- `POST /query` works but is slow (no timeout should be applied)
- `POST /query/data` returns minimal data
- No HTTP `/search` endpoint exists (only MCP tool `search_entities`)
- MCP tools are excellent for discovery but insufficient for implementation (no file read/write)

**Optimization recommendations identified:**
1. Fix `/query` timeout handling (remove timeout or increase to 60s+)
2. Remove `/search` from HTTP endpoint list (it's an MCP tool only)
3. Add `read_file` and `edit_file` MCP tools so the agent can implement changes
4. Optimize `POST /query/data` to return richer structured data
5. Improve C# analyser relation output for denser graph connectivity

### Steps Completed
1. ✅ Run `coderag scan dotnet-console-games` — 452 entities, 587 relations
2. ✅ Dispatch sub-agent with prompt from `docs/subagent-prompts/`
3. ✅ Verify all MCP tools work at scale
4. ✅ Record findings and bugs (see evaluation report)
5. ✅ Create optimization plan for next implementation phase
