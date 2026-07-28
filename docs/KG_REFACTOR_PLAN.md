# Codebase Knowledge Graph Refactoring Plan

## Problem Summary

The current knowledge graph stores every code element (class, method, property, nested type) as
flat, unqualified nodes in an undirected graph. For a small game project this produces ~500 nodes
and ~680 edges — way too dense. For a real codebase it would be unmanageable.

## Root Causes

| # | Problem | Impact |
|---|---------|--------|
| 1 | **Flat entity names** — `Run`, `Ball`, `Console` without class context | Name collisions across classes; graph unreadable |
| 2 | **No call graph** — `extractCalls()` collects call sites but never inserts them | The most useful code relation (who calls whom) is missing |
| 3 | **No hierarchy** — a method node is the same "shape" as a class node | Can't collapse/expand; every detail pollutes the top-level view |
| 4 | **Undirected + single edge** — NetworkX is undirected, `has_edge()` prevents multi-type edges | Can't have both `defines` and `contains` between same pair |
| 5 | **No file/namespace entities** — no source file or package nodes | Can't answer "what's in this file?" structurally |

## End Goal

```
                Layer 0: Files (source files with metadata)
                    │ contains
                Layer 1: Namespaces (package hierarchy)
                    │ contains
                Layer 2: Types (classes, interfaces, structs, enums)
                    │ defines / has / contains
                Layer 3: Members (methods, properties, fields)
                    │ calls / uses
                Layer 4: Call Graph (method→method edges)
```

Each layer is **queryable independently**. An MCP tool can request "just the class view" or
"full detail with call graph".

## Implementation Plan

### Phase 1 — Qualified Entity Names (HIGH priority)

**Goal:** Every entity has a globally unique, human-readable qualified name.

| Change | Where | What |
|--------|-------|------|
| 1.1 | `walker.go:handleMethodDecl` | Entity name → `cls.Name` (was `Name`) |
| 1.2 | `walker.go:handlePropDecl` | Entity name → `cls.Name` (was `Name`) |
| 1.3 | `walker.go:handleClassDecl` | Nested class name → `parent.Name`; top-level unchanged |
| 1.4 | `walker.go:handleMethodDecl/PropDecl` | Relation Target → qualified name (was unqualified) |
| 1.5 | `walker.go:extractCalls` | Callee resolution uses qualified names |
| 1.6 | `parser/csharp.go` | Verify dedup keys still work with qualified names |
| 1.7 | Rebuild `csharp_analyser.exe` + `coderag.exe` | Build + deploy |
| 1.8 | Re-scan test project | Verify output |

### Phase 2 — Call Graph Insertion (HIGH priority)

**Goal:** Method call sites are stored as LightRAG relations.

| Change | Where | What |
|--------|-------|------|
| 2.1 | `output.go:WriteFileResult` | Write `CALL|...|` lines for each `CallSite` |
| 2.2 | `parser/csharp.go` | Parse `CALL|` lines into relations with `calls, depends` keywords |
| 2.3 | `walker.go:handleClassDecl` | Build namespace hierarchy from source path |
| 2.4 | `pipeline/run.go:applyCSharpResults` | Pass call relations through CRUD pipeline |
| 2.5 | Rebuild + re-scan | Verify call edges appear in graph |

### Phase 3 — Multi-Type Edges & Directed Relations (MEDIUM)

**Goal:** Allow multiple relation types between the same pair; store direction metadata.

| Change | Where | What |
|--------|-------|------|
| 3.1 | `parser/csharp.go` | Dedup key → `Source|Target|Keywords` (was `Source|Target`) |
| 3.2 | `pipeline/run.go` | CRUD dedup key → include keywords |
| 3.3 | `utils_graph.py` (LightRAG) | Allow `has_edge` check with relation-type parameter |
| 3.4 | MCP client methods | When fetching neighbors, dedup by type |
| 3.5 | Rebuild Docker, verify | |

### Phase 4 — File Entities & Hierarchy (LOW priority)

**Goal:** Source files and namespaces become queryable graph nodes.

| Change | Where | What |
|--------|-------|------|
| 4.1 | Analyser output | Emit FILE and NAMESPACE entity lines |
| 4.2 | `types.go` | Add `EntityFile`, `EntityNamespace` constants |
| 4.3 | `parser/csharp.go` | Parse FILE/NAMESPACE entity lines |
| 4.4 | MCP `list_entities_by_type` | Support FILE, NAMESPACE types |
| 4.5 | Rebuild + re-scan | Verify file/namespace nodes in graph |

### Phase 5 — Graph Density Optimization (LOW priority)

**Goal:** Top-level view shows only public API; internals are expandable.

| Change | Where | What |
|--------|-------|------|
| 5.1 | Analyser | Skip private/protected members for top-level output; store as nested metadata |
| 5.2 | MCP `get_entity_details` | Support `depth` parameter (1=shallow, 2=members, 3=full) |
| 5.3 | MCP `get_knowledge_graph` | Add `max_depth` filter to limit node count |
| 5.4 | MCP `get_codebase_stats` | Count by visibility (public vs private) |

## Key Design Decisions

1. **Qualified name format:** `Parent.Child` with `.` separator. Namespace-qualified only when
   needed (`Namespace.Class.Method`). For now, class-qualified is sufficient.

2. **Call graph resolution:** Cross-file calls resolved by the existing `GlobalIndex` (two-phase
   analyser). The index maps method names to their declaring classes/namespaces.

3. **Relation vocabulary** (controlled, not free-text):

   | Relation Keywords | Direction | Meaning |
   |-------------------|-----------|---------|
   | `contains, nested` | parent → child | Type contains nested type |
   | `method, defines` | class → method | Class declares method |
   | `property, has` | class → property | Class has property/field |
   | `calls, depends` | caller → callee | Method calls another method |
   | `extends, inherits` | class → base | Class extends base type |
   | `implements` | class → interface | Class implements interface |
   | `contains, file` | file → type | File contains type definition |
   | `contains, namespace` | namespace → type | Namespace contains type |

4. **Downstream compatibility:** The parser (`parser/csharp.go`) and pipeline
   (`pipeline/run.go`, `cmd/coderag/main.go:applyScanResults`) use entity names as opaque
   identifiers. Changing the name format doesn't break them — it only changes the values.

## Files to Modify

### Phase 1
- `analysers/csharp/walker.go` — qualified names in `handleMethodDecl`, `handlePropDecl`, `handleClassDecl`
- `analysers/csharp/types.go` — (if needed)
- `analysers/csharp/output.go` — (if needed)

### Phase 2
- `analysers/csharp/output.go` — emit CALL lines
- `pkg/watchdog/parser/csharp.go` — parse CALL lines
- `pkg/watchdog/pipeline/run.go` — include calls in CRUD
- `cmd/coderag/main.go` — include calls in scan

### Phase 3
- `pkg/watchdog/parser/csharp.go` — type-aware dedup
- `LightRAG/lightrag/utils_graph.py` — type-aware edge check
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/client.py` — type-aware neighbor filter

### Phase 4
- `analysers/csharp/types.go` — FILE, NAMESPACE entity types
- `analysers/csharp/walker.go` — emit file/namespace entities
- `analysers/csharp/output.go` — (if needed)

### Phase 5
- `analysers/csharp/walker.go` — visibility filtering
- `daniel-lightrag-mcp/src/daniel_lightrag_mcp/server.py` — depth parameter
