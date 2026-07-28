# Phase 33 — Graph Entity Model Redesign

> **Status:** ✅ **COMPLETE** — implemented and verified on `dotnet-console-games` (348 files): 154 `extends`, 4 `implements`, 0 NAMESPACE entities, 0 `return`-type property leaks. Two extra pre-existing bugs fixed along the way: multi-modifier `classRegex` (e.g. `public abstract class`) and `global using` handling (cross-namespace type resolution now walks current ns → file usings → global usings → unique simple-name fallback).

## Motivation

The current entity model uses NAMESPACE as a separate top-level node connected to every class via `contains, namespace` edges. This creates visual clutter in the LightRAG WebUI and adds an unnecessary hop when navigating from namespace → class → member. Additionally, inheritance/implements relations and cross-file call resolution via the GlobalIndex are not wired up, leaving the graph with missing structural information.

## New Entity Model

**Top-level nodes** = `<namespace>.<ClassName>` (entity type remains CLASS, INTERFACE, STRUCT, or ENUM):

| Current | New |
|---------|-----|
| CLASS `Calculator` | CLASS `TestLib.Calculator` |
| CLASS `AdvancedCalculator` | CLASS `TestLib.AdvancedCalculator` |
| NAMESPACE `TestLib` | *(removed — namespace is now part of the name)* |

**Child nodes** = `<namespace>.<ClassName>.<MemberName>`:

| Current | New |
|---------|-----|
| METHOD `Calculator.Add` | METHOD `TestLib.Calculator.Add` |
| PROPERTY `DataStore._items` | PROPERTY `TestLib.DataStore._items` |

**Global namespace** (no `namespace` declaration) — names unchanged:

| Current | New |
|---------|-----|
| CLASS `Class1` | CLASS `Class1` (unchanged) |
| METHOD `Class1.Method` | METHOD `Class1.Method` (unchanged) |

**Relations between fully-qualified names:**

- `TestLib.AdvancedCalculator` → `extends, inherits` → `TestLib.Calculator`
- `TestLib.Calculator` → `implements` → `TestLib.IOperation`
- `TestLib.Calculator.SummarizeStore` → `calls, depends` → `TestLib.DataStore.GetAll`
- `TestLib.Calculator` → `method, defines` → `TestLib.Calculator.Add`
- `TestLib.Calculator` → `property, has` → `TestLib.Calculator.Count`
- `TestLib.Outer` → `contains, nested` → `TestLib.Outer.Inner`

---

## Implementation Plan

### Step 1: GlobalIndex — Add Namespace Support

**File:** `analysers/csharp/index.go`

**Changes:**
- Add `Namespace string` field to `ClassInfo` struct
- In `BuildIndex()`: when classifying entities (CLASS/INTERFACE/STRUCT/ENUM), store the entity's namespace alongside its name
- Add a reverse lookup map: `simpleClassName → []fullyQualifiedName` for resolving base types
- Export `BuildInheritanceRelations()` to be callable from the pipeline

**Why first:** The index must carry namespace information before entity names can be correctly prefixed and before inheritance relations can be resolved.

### Step 2: Walker — Namespace-Qualified Entity Names

**File:** `analysers/csharp/walker.go`

**Changes:**

1. **`handleClassDecl`** — prefix entity name with namespace:
   ```go
   namespacePrefix := ""
   if p.namespace != "" {
       namespacePrefix = p.namespace + "."
   }
   fullyQualifiedName := namespacePrefix + qualifiedClassName
   ```
   Push the fully-qualified name onto `classStack` so that all derived method/property names automatically include the namespace.

2. **`handleMethodDecl`** and **`handlePropDecl`** — no changes needed. The `cls` from `currentClass()` already returns the namespace-qualified class name from the stack, so `cls + "." + methodName` naturally becomes `TestLib.Calculator.Add`.

3. **Remove `postProcess()`** — no more NAMESPACE entity emission or `contains, namespace` relations.

4. **`parseImport` / import tracking** — add `using` directive parsing to support cross-namespace base type resolution (if needed for the test project).

### Step 3: Walker — Emit Inheritance/Implements Relations

**File:** `analysers/csharp/walker.go` — `handleClassDecl`

**Changes:**

After parsing the base clause:
```go
// Resolve base type to fully-qualified name
qualifiedBase := ""
if p.namespace != "" {
    candidate := p.namespace + "." + baseName
    if _, ok := p.index.Classes[candidate]; ok {
        qualifiedBase = candidate
    }
}
if qualifiedBase == "" {
    if _, ok := p.index.Classes[baseName]; ok {
        qualifiedBase = baseName
    }
}
if qualifiedBase != "" {
    p.addRelation(Relation{
        Source: entityName,
        Target: qualifiedBase,
        Description: fmt.Sprintf("%s inherits %s", entityName, qualifiedBase),
        Keywords: "extends, inherits",
        Weight: 1.0,
    })
}
```

For interfaces:
```go
qualifiedIface := p.namespace + "." + ifaceName
if _, ok := p.index.Classes[qualifiedIface]; ok {
    p.addRelation(Relation{...Keywords: "implements"...})
}
```

**Note:** The Phase 2 callback `WalkFile` (or equivalent) must pass the GlobalIndex to the walker/parser so `handleClassDecl` can resolve base types. Currently `runScan` builds the index in Phase 1 but discards it (`_ = index`). This needs to be wired.

### Step 4: Walker — Wire Cross-File Call Resolution

**File:** `analysers/csharp/walker.go` — `extractCalls`

**Changes:**

For each `CallSite`, use the GlobalIndex to resolve the callee to a namespace-qualified name:

```go
// After extracting raw callee info from scope vars
if calleeClass != "" && p.index != nil {
    // Try resolving callee class to a known entity in current namespace
    namespacePrefix := ""
    if p.namespace != "" {
        namespacePrefix = p.namespace + "."
    }
    
    // Look up by namespace-qualified name first
    qualifiedCallee := namespacePrefix + calleeClass + "." + callee
    if methods, ok := p.index.Methods[qualifiedCallee]; ok && len(methods) > 0 {
        calleeName = qualifiedCallee
        calleeClass = namespacePrefix + calleeClass
    } else {
        // Fall back to unqualified lookup
        calleeName = calleeClass + "." + callee
    }
}
```

Also filter out calls where `calleeClass` is empty (unresolved framework calls) — don't emit CALL lines for these.

### Step 5: Pipeline — Wire Inheritance + Call Relations

**File:** `analysers/csharp/main.go` (or equivalent entry point)

**Changes:**

In `runScan()`:
1. Pass `index` to the walker/parser in Phase 2 so it can resolve names against the index
2. Call `writeIndex(index)` to output the index file
3. Optionally call `BuildInheritanceRelations` for cross-file inheritance sanity

**File:** `pkg/watchdog/pipeline/run.go`

**Changes:**
The downstream parser already handles `RELATION` lines with arbitrary keywords (`extends, inherits`, `implements`, `calls`, etc.) — no changes needed here. It treats them all as relations with the given keywords.

### Step 6: Clean Up NAMESPACE Entity Type

**File:** `analysers/csharp/types.go`
- Keep `EntityNamespace` constant for backward compat but mark as deprecated, OR remove it entirely

**File:** `pkg/watchdog/parser/csharp.go`
- Remove `"NAMESPACE"` from the entity type switch
- Or keep it for backward compat (it won't match any incoming lines if the analyser stops emitting NAMESPACE)

### Step 7: Fix Field Type Parsing Bug

**File:** `analysers/csharp/walker.go` — property/field regex or handler

Root cause: The property regex likely captures `return` from expression-bodied members. Fix:
```go
// Current regex (likely) matches something like:
// `private int result;` but captures `return` from context
// Fix: tighten the regex to only capture type after access modifier and before field name
```

### Step 8: Rebuild and Test

```powershell
go build -o csharp_analyser.exe ./cmd/csharp_analyser/
copy csharp_analyser.exe analysers\csharp\
go build -o coderag.exe ./cmd/coderag/
```

### Step 9: Verify

```powershell
coderag scan --clean E:\_dev\_Projects\test-csharp-project
```

Check analyser output for:
- Entity names: `TestLib.Calculator`, `TestLib.Calculator.Add`
- Relations: `extends, inherits`, `implements`, `calls`
- No NAMESPACE entities
- No unresolved framework calls
- Correct field types

---

## Relation Vocabulary (Updated)

| Keywords | Direction | Meaning | Phase |
|----------|-----------|---------|-------|
| `contains, nested` | parent → child | Type contains nested type | 32.1 |
| `method, defines` | class → method | Class declares method | 32.1 |
| `property, has` | class → property | Class has property/field | 32.1 |
| `calls, depends` | caller → callee | Method calls another method | 32.2 |
| `extends, inherits` | class → base | Class extends base type | **Phase 33** |
| `implements` | class → interface | Class implements interface | **Phase 33** |
| *(removed)* `contains, namespace` | — | Replaced by namespace prefix in name | Phase 33 |
| *(removed)* NAMESPACE entity | — | Replaced by namespace prefix in name | Phase 33 |

---

## Issues Addressed

| GRAPH_QUALITY_ISSUES.md # | Issue | Fixed By |
|---------------------------|-------|----------|
| 1 | Missing inheritance/implements | Step 3 |
| 2 | Private fields as nodes | *(unchanged — visibility tag still present)* |
| 3 | Partial class duplicates | *(unchanged — dedup handled by CRUD pipeline)* |
| 4 | Framework call noise | Step 4 — filter empty calleeClass |
| 5 | Field type parsing bug | Step 7 |
| 6 | Top-level statements | *(unchanged)* |
| 7 | Self-call loops | Step 4 — callee resolution will resolve or skip |
