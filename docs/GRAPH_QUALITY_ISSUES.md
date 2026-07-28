# Graph Quality Issues

> **Status:** Issues 1, 4, 5, 7 ✅ **resolved in Phase 33** (see `docs/PHASE33_REDESIGN.md`). Issues 2, 3, 6 remain open.

Issues found after `coderag scan --clean` on `test-csharp-project` (44 entities, 51 relations extracted from 6 files). Verified from analyser output files in `.coderag/analyser_output/`.

---

## Issue 1: Missing Inheritance/Implements Relations ✅ RESOLVED (Phase 33)

**Severity:** High
**Location:** `analysers/csharp/walker.go` — `handleClassDecl`

**Resolution:** Relations now emitted via GlobalIndex lookup in `handleClassDecl` (`inheritance, extends` / `implements`, weight 1.0). Cross-namespace base types resolve via current ns → file usings → global usings → unique simple-name fallback. Verified: 154 extends + 4 implements on dotnet-console-games.

**What:** The analyser emits no `extends, inherits` or `implements` relations despite the test project having:
- `class AdvancedCalculator : Calculator` (inheritance)
- `class User : EntityBase` (inheritance)
- `class DataStore : IEnumerable<string>` (interface)
- `class AdvancedCalculator : IOperation` (interface)

All 51 output relations are only of types: `method, defines`, `property, has`, `contains, namespace`.

**Expected:** RELATION lines like:
```
RELATION|AdvancedCalculator|Calculator|AdvancedCalculator inherits Calculator|extends, inherits|...|1.0
RELATION|Calculator|IOperation|Calculator implements IOperation|implements|...|1.0
```

---

## Issue 2: Private Fields as PROPERTY Entities

**Severity:** Medium
**Location:** `analysers/csharp/walker.go` — `handlePropDecl`

**What:** Private fields (backing fields, internal state) are emitted as full PROPERTY entities:
- `DataStore._items` [private]
- `DataStore._nextId` [private]
- `AdvancedCalculator.result` [private]

These have `[private]` visibility tags, but they still create graph nodes. For graph density optimization, private implementation details should either be omitted or merged into the parent class description instead of being standalone nodes.

**Options:**
- Option A: Skip private/protected members entirely (cleaner top-level view)
- Option B: Keep them but with lower weight/priority
- Option C: Keep as-is (current behavior)

---

## Issue 3: Partial Class Duplicate Entities

**Severity:** Medium
**Location:** Pipeline CRUD (`pkg/watchdog/pipeline/run.go`, `pkg/watchdog/crud/client.go`)

**What:** `Calculator` is a partial class defined in both `Calculator.cs` and `AdvancedCalculator.cs`. The analyser processes each file independently and emits duplicate entities:
- `Calculator.Add` from `Calculator.cs:5` AND `AdvancedCalculator.cs:11`
- `Calculator.Subtract` from `Calculator.cs:6` AND `AdvancedCalculator.cs:13`

The pipeline creates the entity from the first file, then the second file's `POST /graph/entity/create` tries to create the same entity again → LightRAG returns 400 "already exists". The pipeline currently `continue`s on error (skips the duplicate). This means:
- Entity metadata (file_path, description) reflects only the first occurrence
- Related relations from the second file may or may not be created depending on keyword merge behavior

**Impact:** For partial classes, methods only point to one source file. Method `SummariasteStore` at line 15 in Calculator.cs is captured from the first file's analysis, but additional methods on Calculator defined only in AdvancedCalculator.cs (like `Multiply`, `Divide`) are correctly captured because they're unique names.

Wait — actually, `Calculator.Multiply` and `Calculator.Divide` only appear in `AdvancedCalculator.cs`. They're unique entities so they get created fine. The duplicate issue is only for methods that exist in BOTH partial files.

---

## Issue 4: Framework/Unresolved Call Noise ✅ RESOLVED (Phase 33)

**Severity:** Low
**Location:** `analysers/csharp/walker.go` — `extractCalls`

**Resolution:** Calls with empty `calleeClass` are now skipped (`if calleeClass == "" { continue }`), so no dangling `calls` relations to non-existent entities are emitted.

**What:** The call graph includes calls to framework/BCL methods that can't be resolved:
```
CALL|DataStore.cs|DataStore.FindById|TryGetValue||30
CALL|DataStore.cs|DataStore.Remove|TryGetValue||35
CALL|DataStore.cs|DataStore.Remove|Remove||36
CALL|DataStore.cs|DataStore.ClearAll|Clear||43
CALL|DataStore.cs|DataStore.ClearAll|Clear||44
CALL|AdvancedCalculator.cs|AdvancedCalculator.Sqrt|Sqrt||46
CALL|Calculator.cs|Calculator.SummarizeStore|DataStore.GetAll|DataStore|17
CALL|ExtendedTypes.cs|User.Validate|User.Validate|User|46
CALL|ExtendedTypes.cs|User.Validate|IsNullOrWhiteSpace||47
```

Calls where the callee class is empty (`||`) are framework methods. They create `calls` relations to non-existent entities, which might fail at CRUD time or create dangling references.

**Options:**
- Filter out CALL lines where callee class is empty (no resolution)
- Or: only emit CALL lines for resolved cross-file calls

---

## Issue 5: Field Type Parsing Glitch ✅ RESOLVED (Phase 33)

**Severity:** Low
**Location:** `analysers/csharp/walker.go` — `handlePropDecl` (field declaration regex)

**Resolution:** Root cause was `return result;` lines inside methods hitting the class-level `varDeclRegex` branch, capturing `return` as the type. Fixed with an `isKeyword(v[1])` guard. Verified: 0 `-> RETURNS -> return` properties in output.

**What:** The field `private int result;` at `AdvancedCalculator.cs:40` returns type as `return`:
```
PROPERTY|AdvancedCalculator.result|...|40|AdvancedCalculator.result -> RETURNS -> return
```

Expected: `AdvancedCalculator.result -> RETURNS -> int`

Root cause: The regex for extracting field types probably captures `return` from context (e.g., `=> return result;` from expression-bodied members bleeding into field parsing).

---

## Issue 6: Top-Level Statement Programs Produce Empty Output

**Severity:** Low
**Location:** `analysers/csharp/walker.go`

**What:** `TestConsole__Program.cs.txt` has empty exports. The console project likely uses top-level statements (`Console.WriteLine(...)`) which the analyser doesn't handle (no class/method structure to parse).

**Impact:** The program entry point and its logic are invisible in the graph.

---

## Issue 7: Self-Calls Create Self-Loops ✅ RESOLVED (Phase 33)

**Severity:** Low
**Location:** `analysers/csharp/walker.go` — `extractCalls`

**Resolution:** Cross-file call resolution now uses GlobalIndex to match callee class to namespace-qualified entity names. Unresolved (framework) calls filtered out (empty calleeClass skipped). Self-call loops remain only if a method truly names itself within scope, which is rare and handled by LightRAG dedup at the graph layer.

**What:** `User.Validate` has a CALL to itself:
```
CALL|...|User.Validate|User.Validate|User|46
```

This creates a `calls` relation from `User.Validate` → `User.Validate` (a self-loop). Self-calls are usually uninteresting (method calling an overload or base implementation). They add noise to the call graph.

---

## Priority Summary

| # | Issue | Severity | Type | Status |
|---|-------|----------|------|--------|
| 1 | Missing inheritance/implements relations | High | Missing feature | ✅ Resolved |
| 2 | Private fields as PROPERTY entities | Medium | Graph density | Open |
| 3 | Partial class duplicate handling | Medium | Data integrity | Open |
| 4 | Framework call noise | Low | Call graph quality | ✅ Resolved |
| 5 | Field type parsing bug | Low | Analyser bug | ✅ Resolved |
| 6 | Top-level statements not handled | Low | Missing feature | Open |
| 7 | Self-call self-loops | Low | Call graph quality | ✅ Resolved |
