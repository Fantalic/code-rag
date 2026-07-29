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
| Vector DB consistency after cascade | Need to verify entity/relation embeddings are cleaned up on delete | 📋 Pending |

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
- `parseRelationLine` reads `parts[5]` (the file path from `RELATION|...|FilePath|Weight`)
- `ParseCSharpFile` sets `rel.SourceID = filename` for relations (like entities)
- `parserRelationsToMap` includes `"source_id"` and `"file_path"` in the data map

### Tests (12 test cases, all pass)
- **Parser (6 tests):** `TestParseCSharpFile` (SourceID/FilePath/fields), `TestParseCSharpFileEmpty`, `TestParseRelationLineEdgeCases` (5 subtests: missing/invalid/empty/CALL), `TestParseCSharpOutputDedup` (entity dedup), `TestParseCSharpOutputDedupRelations` (relation dedup)
- **Pipeline (3 tests):** `TestParserRelationsToMap` (source_id/file_path output + backward compat), `TestLoadCurrentRelationsParsing` (edge JSON parsing, skip-empty, keyword extraction), `TestRunCSharpAnalyser` (integration: real analyser on test-csharp-project → 42 entities, 40 relations, all 38 non-CALL relations have SourceID/FilePath ✅)
- **Cascade (1 test, 5 subtests):** `TestFindAffectedFilesFromData` — cascade detection, no-cascade, wrong-source_id fallback, exclude-path, additive paths

## Pending

### Step 3: Verify vector DB consistency
1. Query LightRAG `/query/text` before and after deletion to confirm embeddings are removed
2. Verify no stale embeddings remain for deleted entities/relations
