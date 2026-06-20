# Phase 18 Complete — Project-Based Storage & GraphML Fix

## What Was Done

### 1. Project-Based Storage (`.coderag/rag_storage`)

**Problem**: LightRAG stored all data in `./rag_storage` relative to the repo root. Each project folder should have its own isolated storage.

**Solution**:
- Added `find_project_root()` in `config.py` — searches from cwd upward for `.coderag` directory
- Added `get_default_working_dir()` — returns `.coderag/rag_storage` if found, else legacy `./rag_storage`
- Watchdog `start` command sets `LIGHTRAG_WORKING_DIR` env var and `lightragProcess.Dir` to the project folder
- Created `scripts/init-project.ps1` for easy project initialization

**Files Modified:**
- `LightRAG/lightrag/api/config.py` — `find_project_root()`, `get_default_working_dir()`
- `watchdog/cmd/watchdog/main.go` — LIGHTRAG_WORKING_DIR env + Dir setting
- `scripts/init-project.ps1` — new file

### 2. GraphML NoneType Error Fix

**Problem**: `nx.write_graphml()` raises "GraphML does not support type `<class 'NoneType'>` as data values" when node/edge attributes contain None. The graph was written to disk as 0 bytes.

**Root Cause**: Watchdog's entity edit endpoint sent data with `null` values for some fields (e.g., `file_path`, `methods`), which persisted as None in NetworkX graph attributes.

**Solution** (three layers of defense):
1. `upsert_node()` — filters None before `graph.add_node()`
2. `upsert_edge()` — filters None before `graph.add_edge()`
3. `write_nx_graph()` — pre-processes all nodes and edges before serialization

**Files Modified:**
- `LightRAG/lightrag/kg/networkx_impl.py` — None filtering in upsert/write

### 3. No `.env` Requirement

**Problem**: LightRAG refused to start without `.env` file, or prompted for confirmation (which blocked subprocess startup).

**Solution**: `check_env_file()` now always returns `True`. Single-instance deployment doesn't need multi-instance safety.

**Files Modified:**
- `LightRAG/lightrag/api/utils_api.py` — simplified `check_env_file()`

### 4. Test Project Setup

**Problem**: Watchdog used git-based change detection but test-csharp-project was inside the main repo, causing git to see parent repo files.

**Solution**: Moved test project to `E:\_dev\_Projects\test-csharp-project` (outside repo) with its own git repo.

## End-to-End Test Results

### Fresh System Startup
```
watchdog start <project-path>
  -> Starts LightRAG with LIGHTRAG_WORKING_DIR=<project>/.coderag/rag_storage
  -> Full scan of project (6 C# files)
  -> C# analyser extracts classes, methods, properties, inheritance
  -> Graph: 29 nodes, 25 edges
```

### Complex Change — Add IRepository<T> Interface + Methods
- Modified `DataStore.cs`: added `IRepository<T>` interface, `Add()`, `FindById()`, `Remove()` methods
- Watchdog re-analysed entire solution
- Graph: 31 nodes, 28 edges (2 new nodes, 3 new edges)
- ✅ Relations created: DataStore → Remove
- ✅ Vector embeddings generated for new entities
- ✅ No GraphML errors

### Incremental Change — Add ClearAll Method
- Modified `DataStore.cs`: added `ClearAll()` method
- Graph: 32 nodes, 29 edges (1 new node, 1 new edge)

### Verified Features
- ✅ Project isolation (data in project's `.coderag/rag_storage`)
- ✅ Server starts without `.env`
- ✅ GraphML persistence (14KB valid XML)
- ✅ Vector DB populated (240KB)
- ✅ Relations correct directions (not alphabetically swapped)
- ✅ Cross-file relations (inheritance, interface implementation)
- ✅ Git watcher detects uncommitted changes

## Remaining Issues

| Issue | Priority | Notes |
|-------|----------|-------|
| Stale entities not cleaned up | Medium | Old entities persist if file is deleted while watchdog not running |
| Git watcher only detects uncommitted changes | Low | Poll cycle is 30s. New commits need restart. |
| IRepository<T> not extracted | Low | C# analyser doesn't extract generic interfaces |
| No periodic sweep | Low | Would remove orphaned entities |
