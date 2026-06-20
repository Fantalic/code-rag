# Phase 20 — VDB Embedding & file_path Persistence ✅ COMPLETE

## Issues Found & Fixed

### Issue 1: Entity `file_path` lost in VDB after update
- **Root Cause**: `aedit_entity()` at `LightRAG/lightrag/utils_graph.py:337` re-built the VDB upsert dict without `file_path`. Since the diff engine emits `ActionUpdate` for every entity after the first `ActionCreate` (multiple .cs files share classes), the edit path **overwrote** the VDB entry created by `acreate_entity()`.
- **Fix**: Added `"file_path": new_node_data.get("file_path", "")` to the VDB upsert dict.
- **Verification**: Direct API test confirmed `file_path` persists after create → update flow.

### Issue 2: Relation `file_path` lost in VDB after update
- **Root Cause**: Same pattern — `aedit_relation()` at `utils_graph.py:453` omitted `file_path` from VDB upsert.
- **Fix**: Added `"file_path": new_edge_data.get("file_path", "")` to the VDB upsert dict.
- **Verification**: Same create → update flow test for relations.

## Tests Added
**`tests/test_integration_graph_vdb.py`** — 11 integration tests:
- `TestEntityVDB` (4 tests): create entity → VDB has file_path + vector ✅ ; update preserves file_path ✅ ; no file_path defaults ✅ ; rename preserves vector ✅
- `TestRelationVDB` (3 tests): create relation → VDB has file_path + vector ✅ ; update preserves file_path ✅ ; no file_path defaults ✅
- `TestGraphEmbeddingIntegrity` (4 tests): full workflow (3 entities + 3 relations, create + update) ✅ ; update entity/relation without file_path retains graph-stored value ✅

## Test Infrastructure
- Uses session-scoped mocked FastAPI server (`tests/conftest.py`) with mocked Ollama LLM + embedding
- All tests use `httpx.AsyncClient` with `ASGITransport`
- Embedding mock returns `np.array([[0.1]*1024, ...])` — sufficient to verify VDB upsert behavior
- Vector DB state verified through `include_vector_data=True` response from API endpoints

## Files Modified
- `LightRAG/lightrag/utils_graph.py` — added `file_path` to VDB upsert in both `aedit_entity` and `aedit_relation`
- `tests/test_integration_graph_vdb.py` — new test file (11 tests)
