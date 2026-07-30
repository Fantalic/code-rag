# Future Ideas & Optimizations

## Analyser-to-Graph Direct Memory Stream

### Concept
Skip the intermediate file-based communication between analysers and the graph storage. Currently:

1. Analyser (e.g., C# walker) writes `.txt` output files to disk
2. coderag reads those files
3. Parser parses text → entities/relations
4. Diff engine compares with existing data
5. CRUD client sends HTTP POSTs to LightRAG API
6. LightRAG upserts into graph storage → saves to disk

The direct memory stream would connect analyser output directly to graph insertion:

```
Analyser (in-process Go) → Entities/Relations structs → Batch CRUD client → LightRAG API
```

Skipping steps 1-3 (file write, file read, text parse).

### When to Revisit
- Current bottleneck is the API round-trips per entity/relation, which the Batch Import API (v1) already fixes by reducing 3576 calls to 2.
- Direct memory stream becomes valuable when:
  - Analyser runtime is a significant portion of total scan time (currently ~5s for 1701 entities → negligible)
  - Adding more analysers (Python, JS, Rust, etc.) where subprocess overhead dominates
  - Real-time streaming feedback required (progressive graph updates during scan)

### Potential Architecture
```go
// In-process analyser returns structured data directly
type AnalyserResult struct {
    Entities  []parser.Entity
    Relations []parser.Relation
}

// Pipeline pushes directly to CRUD client
func RunAnalyser(sln string) (*AnalyserResult, error) {
    // walker walks AST, populates structs directly
    // no file I/O needed
}

// CRUD client batches and sends
result, _ := analyser.Run(sln)
client.BatchCreateEntities(result.Entities)
client.BatchCreateRelations(result.Relations)
```

### Trade-offs
- ✅ Eliminates all file I/O for analyser output
- ✅ Removes parser step (text → struct is now direct struct construction)
- ✅ Enables streaming (emit entities/relations as they're discovered)
- ❌ Ties analyser tightly to Go pipeline (harder to add non-Go analysers)
- ❌ Loses debuggable artefact (analyser_output/*.txt files for inspection)
- ❌ More complex analyser implementation (must know Go struct layout)

### File-based is a Feature, Not a Bug
The `.txt` output files serve as:
- Debugging artefacts (inspect what the analyser found)
- Caching (re-run parser without re-running analyser)
- Language-agnostic analyser interface (any language can write `|`-delimited lines)

Keep file-based as the primary interface. Add direct memory as an opt-in fast path when analyser count grows.

## Batch Import API (v1 - Implemented)

- `POST /graph/entities/batch-create` — creates N entities with 1 save
- `POST /graph/relations/batch-create` — creates N relations with 1 save (skips entity existence validation)
- See `graph_routes.py` for implementation
