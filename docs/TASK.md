# CRUD Pipeline Adaptation — Phase 33 Format Changes

## Current Task

The C# analyser output format changed in Phase 33 (namespace-qualified entity names like `Console_Monsters.Bases.CharacterBase`, new relation types `extends, inherits` and `implements`, framework call filtering). The downstream CRUD pipeline that reads these files and creates entities/relations in LightRAG has bugs preventing correct processing.

## What's Broken

| Issue | Description | Evidence |
|-------|-------------|----------|
| Silent error swallowing | `applyScanResults` (main.go:596-649) swallows all entity/relation creation errors — no logging | 906 entities fail, 0 relations created, zero error output |
| 906/1701 entities fail creation | ~53% of entities silently fail during creation in LightRAG | Analyser extracts 1701, only 795 created in graph |
| 0/1743 relations created | ALL relation creations fail silently | 1743 extracted, 0 created |
| `ClearAll()` incomplete | `DELETE /documents` doesn't purge graph storage from `rag_data` volume | Stale pre-Phase 33 entities survive across rebuilds |

## Plan

### Step 1: Fix silent error swallowing in `applyScanResults` (main.go)
- Add `log.Printf` for failed entity creation with error details
- Add `log.Printf` for failed relation creation with error details
- This will expose the actual root causes of failures

### Step 2: Fix `applyCSharpResults` in `run.go` to match patterns
- Apply same error visibility improvements as `applyScanResults`
- Ensure consistent diff/create/update handling

### Step 3: Fix `ClearAll()` to purge graph storage
- `DELETE /documents` only clears document metadata, not graph nodes/edges
- Need to also purge `rag_data` volume or call LightRAG's graph-clear endpoint
- Alternative: delete the `rag_data` volume directly in the `coderag scan --clean` flow

### Step 4: Verify clean rescan works correctly
- Purge `rag_data` volume (`docker compose down -v`)
- Rebuild Docker image
- Run `coderag scan --clean` on `dotnet-console-games`
- Check entity count (should be 1701 or close)
- Check relation count (should be 1743 or close)
- Verify namespace-qualified names are stored correctly
- Verify extends/implements relations exist (154+4)

### Step 5: Update documentation
- Update CURRENT_CONTEXT.md with findings
- Update PLAN.md with Phase 27.5 status
- Clean up outdated historical sections

## Phase 33 — Graph Entity Model Redesign ✅ COMPLETE

All steps implemented and verified (see `docs/PHASE33_REDESIGN.md`).

**Verified on `dotnet-console-games` (348 files):** 154 `extends` + 4 `implements` relations, 0 NAMESPACE entities, 0 `return`-type leaks. Commit: `1330e381`.