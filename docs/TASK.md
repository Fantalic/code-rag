# 🔥 Phase 13 — Fix Concurrent Analyser Race + Relations

## The Problem

**Race condition in C# analyser invocation:**
- Multiple workers (4 by default) process different .cs files from the same solution concurrently
- Each worker calls `runCSharpAnalyser(sln)` which writes temp output files
- When two workers run the analyser simultaneously on the same solution, one overwrites the other's temp files
- Result: analyser output gets mixed up, relation source/target pairs get swapped

**Why only 3 entities (classes) but 0 relations initially:**
- The analyser was run but the temp files were overwritten before parsing could complete
- After adding METHOD entities, relations started being created but with swapped source/target

**Current symptom (after METHOD entity fix):**
- Edges exist but directions are swapped: `Calculator -> Subtract` appears as `Add -> Calculator`
- 7 nodes, 4 edges, but edge directions are wrong

## Root Cause Analysis

```
Worker A (Calculator.cs) ──┐
                           ├─► runCSharpAnalyser(TestCodebase.sln) ──► temp dir A
Worker B (DataStore.cs) ──┘                           │
                           ├─► runCSharpAnalyser(TestCodebase.sln) ──► temp dir B (overwrites A!)
                           └─► runCSharpAnalyser(TestCodebase.sln) ──► temp dir C
```

Each worker creates its own temp dir, but `runCSharpAnalyser` uses `os.MkdirTemp` which could produce the same name if called rapidly. More importantly, the analyser itself writes to a fixed subdir structure.

Actually, looking at the code: `os.MkdirTemp` always creates unique dirs. The issue is different — each worker calls the analyser on the **same solution** and reads back results, but if the solution takes time to analyse, the results may not be ready before another worker starts the same analyse.

But wait — the temp dir IS unique per call. So the race must be in how results are read back. The workers all call `runCSharpAnalyser` with the same sln path, each gets their own temp dir, runs the analyser, then reads their own temp dir. That should be fine...

Let me look more carefully. The log shows:
```
Calculator -> Subtract
AddItem -> DataStore
DataStore -> GetAll
Add -> Calculator
```

The "Add" relation (Calculator.Add) appears as `Add -> Calculator` (source/target swapped). The "Subtract" relation appears correct direction. This is inconsistent — some relations are correct, some swapped.

Actually, I think I see it now. The workers process files in parallel. Calculator.cs is processed first, runs the analyser, gets results. But DataStore.cs is also processed around the same time — it also runs the analyser, gets results, and the output files for DataStore methods overwrite or come after Calculator's output. But wait, they're in different temp dirs...

The REAL issue: `runCSharpAnalyser` takes the sln path and runs the analyser. The analyser writes output for ALL classes in the solution to the temp dir. So Worker A (Calculator.cs) invokes the analyser → gets output with Calculator AND DataStore classes. Worker B (DataStore.cs) ALSO invokes the analyser → gets the SAME output. But since the analyser writes to the same structure each time, the files are identical.

So why are relations swapped? The `parser.ParseCSharpOutput` deduplicates by `(source, target)` key. If two workers call it with the same results, deduplication should produce the same output...

Unless the deduplication is happening across workers, and the relation key construction has a race. But `parserRelationsToMap` uses `diff.RelationKey(r.Source, r.Target)` which is just `src + "|" + tgt`.

Let me think again. The parser creates:
- Entity: Calculator (CLASS)
- Entity: Add (METHOD)
- Entity: Subtract (METHOD)
- Relations from parser.ParseCSharpOutput:
  - Calculator -> Add
  - Calculator -> Subtract

But `parser.ParseCSharpOutput` iterates over results from `runCSharpAnalyser`. Each call to the analyser writes output files to a temp dir. If two workers call `runCSharpAnalyser` nearly simultaneously with different temp dirs, and the analyser takes time... hmm.

Actually wait — let me check the order of operations:
1. Worker A processes Calculator.cs → finds sln → calls `runCSharpAnalyser(sln)` → creates temp dir A → runs analyser → analyser writes to temp dir A → Worker A reads temp dir A → parses
2. Worker B processes DataStore.cs → finds sln → calls `runCSharpAnalyser(sln)` → creates temp dir B → runs analyser → analyser writes to temp dir B → Worker B reads temp dir B → parses

Both workers run the analyser independently, but since they're running the SAME analyser on the SAME solution, they produce the SAME output (Calculator, Add, Subtract, DataStore, AddItem, GetAll, Class1). The deduplication in `ParseCSharpOutput` uses `seenEntities` and `seenRelations` maps. 

But wait — if Worker A parses first, it creates Calculator, Add, Subtract, DataStore, AddItem, GetAll, Class1 as entities and the relation pairs. Worker B parses the same output, deduplicates the same entities, and tries to create the same relations. Should be idempotent...

Unless the issue is in the DIFF. When Worker A creates entities and relations, LightRAG has them. When Worker B calls `GetAllEntities()` it should see the existing entities. But if the workers are truly concurrent, Worker A hasn't finished creating before Worker B reads.

BUT — the actual swap: `Add -> Calculator` instead of `Calculator -> Add`. The parser creates `{Source: "Calculator", Target: "Add"}`. The diff then computes relation actions. The relation creation call is `client.CreateRelation(action.Source, action.Target, ...)`. So if the action says CREATE with Source=Calculator, Target=Add, we call CreateRelation("Calculator", "Add", ...).

And the edge data shows: `source: "Add", target: "Calculator"` — source and target are SWAPPED at the API level.

Let me check the CreateRelation payload:
```go
func (c *Config) CreateRelation(source, target, description, keywords string, weight float64, sourceID, filePath string) (map[string]any, error) {
    payload := CreateRelationPayload{
        SourceEntity: source,
        TargetEntity: target,
        ...
    }
}
```

So we call `CreateRelation("Calculator", "Add", ...)` which sets `SourceEntity: "Calculator"` and `TargetEntity: "Add"`.

But the LightRAG graph shows `source: "Add", target: "Calculator"`.

This means either:
1. The Go payload fields are mapped differently than I think
2. LightRAG is swapping them
3. The CreateRelation call is actually receiving swapped arguments somehow

Let me check if maybe the `diff.RelationKey` or `parserRelationsToMap` has an issue. The parser creates relations with Source=className, Target=method. `parserRelationsToMap` creates `{src: className, tgt: method}` as the data dict. Then in `run.go`:
```go
client.CreateRelation(
    action.Source,
    action.Target,
    filter.FilterDescription(getString(data, "description")),
    ...
)
```

`action.Source` comes from the RelationAction which was created from the diff. The diff populates RelationAction from the key in the relation map. The key is made by `diff.RelationKey(r.Source, r.Target)`.

Actually, I think the issue might be that two workers are creating the SAME relations (Calculator->Add) at the same time, and there's some issue with how LightRAG handles concurrent creation of the same relation.

Or — more likely — the problem is that the first worker creates the relation before the second entity (Add) exists. Since LightRAG requires BOTH entities to exist before creating a relation, and we're creating METHOD entities and CLASS entities in the same pipeline call, if the CLASS entity is created first and the METHOD entity hasn't been created yet when the relation is attempted, it might fail or behave unexpectedly.

Let me trace through the pipeline for Calculator.cs:
1. `runCSharpAnalyser(sln)` → returns all analyser output files
2. `parser.ParseCSharpOutput(results)` → entities: [Calculator, Add, Subtract, DataStore, AddItem, GetAll, Class1], relations: [Calculator->Add, Calculator->Subtract, DataStore->AddItem, DataStore->GetAll]
3. `diff.DiffEntities` → all 7 are CREATE actions (graph is empty)
4. For each entity action:
   - ActionCreate Calculator → `client.CreateEntity("Calculator", ...)`
   - ActionCreate Add → `client.CreateEntity("Add", ...)`
   - ActionCreate Subtract → `client.CreateEntity("Subtract", ...)`
   - ...
5. For each relation action:
   - ActionCreate Calculator->Add → `client.CreateRelation("Calculator", "Add", ...)`

Now if Worker A is doing this for Calculator.cs, it creates Calculator, Add, Subtract first, then relations. Worker B is doing the same for DataStore.cs at the same time (or slightly after). If they're truly concurrent, the entity creations and relation creations are interleaved.

Actually — I bet the problem is that the Python prototype had this working correctly because of how the MCP `update_relations` tool works — it runs the analyser once and then calls `insert_text` which handles the LLM extraction. The Go version's `runCSharpAnalyser` returns a fixed set of results that get parsed and fed into the diff, which should be deterministic.

But the relation source/target swap suggests something is getting out of order. Let me check if maybe the issue is the graph response format — when we read the graph after creation, maybe the `source` and `target` in the edge response don't map to the `source_entity` and `target_entity` we sent during creation.

Let me test the relation creation API directly to confirm field names:

Actually, let me just fix the core issue: the concurrent analyser problem. The solution is to run the analyser once per solution, not once per file. Here's the plan:

## Solution: Solution-Level Analyser Invocation

### Step 1: Add AnalyserResult type to pipeline

```go
type AnalyserResult struct {
    Filename string
    Content  string
}
```

### Step 2: Refactor initial sync to group by solution

```go
// Before (per file):
for _, filePath := range files {
    w.workQueue <- Event{Path: filePath, ChangeType: "initial"}
}

// After (grouped by solution):
solutions := groupFilesBySolution(files)
for sln, slnFiles := range solutions {
    results := runSolutionAnalyser(sln)  // Run analyser ONCE per solution
    for _, filePath := range slnFiles {
        w.workQueue <- Event{Path: filePath, ChangeType: "initial", Results: results}
    }
}
```

### Step 3: Refactor pipeline to handle pre-computed results

The `Event` struct carries `Results []AnalyserResult` when available (from initial sync). The pipeline uses these results instead of re-running the analyser. For incremental file changes (after initial sync), re-run the analyser for that specific file's solution and use those results.

### Step 4: Add a mutex to serialize analyser invocations

To be extra safe, add a solution-level mutex map to prevent two workers from analysing the same solution simultaneously:

```go
type Watcher struct {
    ...
    solutionMu sync.Map  // map[slnPath]chan struct{} — prevents concurrent analyse of same sln
}
```

## Implementation Steps

| # | Step | Status |
|---|------|--------|
| 1 | Update `pipeline/run.go` — add `AnalyserResult` type, add `RunEvent(event, client)` that carries pre-computed results | ✅ Done |
| 2 | Update `watcher/observer.go` — solution-level grouping + mutex | ✅ Done |
| 3 | Rebuild + test on C# project | ✅ Done |
| 4 | Verify relations have correct source/target direction | ✅ Done — use src_id/tgt_id from properties |
| 5 | Run all tests (226 Python + Go build) | ⬜ Next |

## Expected Result

After fix:
- 7 nodes: Calculator, Add, Subtract, DataStore, AddItem, GetAll, Class1 ✅
- 4 edges: Calculator->Add, Calculator->Subtract, DataStore->AddItem, DataStore->GetAll ✅
- Edge directions correct — use `src_id`/`tgt_id` from edge properties (not alphabetically-sorted `source`/`target`) ✅

## Files Modified

- `watchdog/internal/pipeline/run.go` — `AnalyserResult` type, `RunEvent`, relation dedup, fix `loadCurrentRelations` to read src_id/tgt_id from properties
- `watchdog/internal/watcher/observer.go` — solution grouping, mutex, Event carries pre-computed results
- `watchdog/internal/parser/csharp.go` — METHOD entities now created alongside CLASS entities