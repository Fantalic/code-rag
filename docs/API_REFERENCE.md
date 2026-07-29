# LightRAG API Reference

> Detailed request/response schemas for all LightRAG endpoints. Referenced from `DOCUMENTATION.md`.

---

## 1. Document Endpoints

### `POST /documents/text`

Insert a single text document into LightRAG.

```json
// Request
{
  "text": "File content here...",
  "file_source": "path/to/file.cs"
}

// Response - success
{
  "status": "success",
  "message": "Text successfully received...",
  "track_id": "insert_20250729_170612_abc123"
}

// Response - duplicated
{
  "status": "duplicated",
  "message": "File source 'path/to/file.cs' already exists...",
  "track_id": ""
}
```

**Dedup mechanism** (`document_routes.py:1824-1840`):
On `POST /documents/text`, the server checks `rag.doc_status.get_doc_by_file_path(request.file_source)`.
If a document with that `file_source` already exists, it returns `status: "duplicated"` with empty `track_id`.

### `POST /documents/texts`

```json
// Request
{
  "texts": ["text1", "text2"],
  "file_sources": ["source1.txt", "source2.txt"]
}
```

### `GET /documents/pipeline_status`

```json
// Response
{
  "autoscanned": false,
  "busy": false,
  "job_name": "Default Job",
  "job_start": null,
  "docs": 0,
  "batchs": 0,
  "cur_batch": 0,
  "request_pending": false,
  "latest_message": "",
  "history_messages": [],
  "update_status": {}
}
```

### `DELETE /documents`

Clears ALL documents, entities, relations, and files. Destructive.

---

## 2. Query Endpoints

### `POST /query`

```json
// Request
{
  "query": "What does MyClass do?",
  "mode": "hybrid",
  "only_need_context": false
}

// Response
{
  "response": "...",
  "results": [...]
}
```

Modes: `naive`, `local`, `global`, `hybrid`.

---

## 3. Entity CRUD Endpoints

### 3.1 Create Entity

**POST** `/graph/entity/create`

Internal: `utils_graph.py:491` — `acreate_entity()`

```json
// Request
{
  "entity_name": "MyClass",
  "entity_data": {
    "description": "Class that handles user authentication",
    "entity_type": "CLASS",
    "source_id": "project/src/auth.py",
    "file_path": "project/src/auth.py"
  }
}

// Response (200)
{
  "status": "success",
  "message": "Entity 'MyClass' created successfully",
  "data": {
    "entity_name": "MyClass",
    "source_id": "project/src/auth.py",
    "graph_data": {
      "entity_id": "MyClass",
      "entity_type": "CLASS",
      "description": "Class that handles user authentication",
      "source_id": "project/src/auth.py",
      "file_path": "project/src/auth.py",
      "created_at": 1719000000
    },
    "vector_data": { ... }
  }
}

// Error (400) - already exists
{
  "detail": "Entity 'MyClass' already exists"
}
```

**What happens internally:**

1. Acquires `graph_db_lock` for concurrency safety
2. Checks if entity exists via `has_node()` — raises `ValueError` if so
3. Prepares node data with defaults: `entity_type` = "UNKNOWN", `source_id` = "manual_creation", `created_at` = now
4. `chunk_entity_relation_graph.upsert_node(entity_name, node_data)` — graph DB
5. Computes `entity_id = md5hash(entity_name, prefix="ent-")` — vector DB key
6. `entities_vdb.upsert(entity_data)` — vector embedding
7. Embedding content = `"{entity_name}\n{description}"`
8. VDB fields stored: `entity_name`, `source_id`, `description`, `entity_type`, `file_path`
9. `_edit_entity_done()` — persists all storage backends
10. Returns result via `get_entity_info()`

**`vector_data` response fields:**
```json
{
  "__id__": "ent-<md5hash>",
  "__created_at__": 1719000000,
  "content": "MyClass\nClass that handles user authentication",
  "entity_name": "MyClass",
  "source_id": "project/src/auth.py",
  "description": "Class that handles user authentication",
  "entity_type": "CLASS",
  "file_path": "project/src/auth.py"
  // "vector" field omitted — binary blob, not returned in API responses
}
```

---

### 3.2 Update Entity

**POST** `/graph/entity/edit`

Internal: `utils_graph.py:163` — `aedit_entity()`

```json
// Request - update properties
{
  "entity_name": "MyClass",
  "updated_data": {
    "description": "Updated: handles auth + rate limiting"
  },
  "allow_rename": false
}

// Request - rename entity
{
  "entity_name": "MyClass",
  "updated_data": {
    "entity_name": "MyClassV2"
  },
  "allow_rename": true
}

// Response (200)
{
  "status": "success",
  "message": "Entity updated successfully",
  "data": { ... }
}

// Error (400)
{
  "detail": "Entity 'MyClass' does not exist"
}
```

**What happens internally (no rename):**
1. Merges `node_data` (from graph DB) + `updated_data` (from request)
2. `chunk_entity_relation_graph.upsert_node(entity_name, merged_data)` — graph DB
3. Recomputes embedding: `content = entity_name + "\n" + description`
4. `entities_vdb.upsert(entity_data)` — **includes `file_path` from merged graph node data** (fix: both `aedit_entity` and `aedit_relation` previously omitted `file_path`, overwriting VDB entries without it)
5. `_edit_entity_done()` to persist

**Rename behavior:**
- Creates new node with new name
- Copies all edges (relations) to new node
- Deletes old node
- Deletes old embeddings from `entities_vdb` AND `relationships_vdb`
- Recalculates all relation embeddings for moved edges
- Calls `_edit_entity_done()` to persist

---

### 3.3 Delete Entity

**DELETE** `/documents/delete_entity`

Internal: `utils_graph.py:14` — `adelete_by_entity()`

```json
// Request
{
  "entity_name": "MyClass"
}

// Response (200)
{
  "status": "success",
  "doc_id": "MyClass",
  "message": "Entity 'MyClass' and its 3 relationships have been deleted.",
  "status_code": 200
}

// Not found (404)
{
  "status": "not_found",
  "doc_id": "MyClass",
  "message": "Entity 'MyClass' not found.",
  "status_code": 404
}
```

**What happens:**
1. Checks `has_node()` — returns 404 if not found
2. Gets all edges via `get_node_edges()` to count related relations
3. Deletes: `entities_vdb.delete_entity()` -> `relationships_vdb.delete_entity_relation()` -> `chunk_entity_relation_graph.delete_node()`
4. `_delete_by_entity_done()` to persist

---

### 3.4 Check Entity Exists

**GET** `/graph/entity/exists?name=MyClass`

```json
// Response
{
  "exists": true
}
```

Internal: `chunk_entity_relation_graph.has_node(name)`

---

## 4. Relation CRUD Endpoints

### 4.1 Create Relation

**POST** `/graph/relation/create`

Internal: `utils_graph.py:575` — `acreate_relation()`

```json
// Request
{
  "source_entity": "MyClass",
  "target_entity": "UserService",
  "relation_data": {
    "description": "MyClass depends on UserService",
    "keywords": "dependency, import, calls",
    "weight": 1.0,
    "source_id": "project/src/auth.py",
    "file_path": "project/src/auth.py"
  }
}

// Response (200)
{
  "status": "success",
  "message": "Relation created successfully between 'MyClass' and 'UserService'",
  "data": {
    "src_entity": "MyClass",
    "tgt_entity": "UserService",
    "graph_data": {
      "description": "MyClass depends on UserService",
      "keywords": "dependency, import, calls",
      "source_id": "project/src/auth.py",
      "weight": 1.0,
      "file_path": "project/src/auth.py",
      "created_at": 1719000000
    },
    "vector_data": { ... }
  }
}

// Error (400) - entity doesn't exist
{
  "detail": "Source entity 'MyClass' does not exist"
}

// Error (400) - already exists
{
  "detail": "Relation from 'MyClass' to 'UserService' already exists"
}
```

**Prerequisites:**
- Both `source_entity` and `target_entity` must already exist in the graph
- Create entities first via `POST /graph/entity/create`

**What happens internally:**
1. Acquires `graph_db_lock`
2. Validates both entities exist via `has_node()` — raises if not
3. Checks if relation already exists via `has_edge()` — if so, **merges keywords** (comma-joins new keywords to existing) instead of raising
4. Prepares edge data with defaults: `weight = 1.0`, `source_id = "manual_creation"`
5. `chunk_entity_relation_graph.upsert_edge(src, tgt, edge_data)` — graph DB
6. Computes `relation_id = md5hash(src + tgt, prefix="rel-")` — vector DB key
7. Embedding content = `"{keywords}\t{src}\n{tgt}\n{description}"`
8. `relationships_vdb.upsert(relation_data)` — vector DB
9. `_edit_relation_done()` — persists

---

### 4.2 Update Relation

**POST** `/graph/relation/edit`

Internal: `utils_graph.py:375` — `aedit_relation()`

```json
// Request
{
  "source_id": "MyClass",
  "target_id": "UserService",
  "updated_data": {
    "description": "Updated dependency description",
    "keywords": "dependency, injects",
    "weight": 2.0
  }
}

// Response (200)
{
  "status": "success",
  "message": "Relation updated successfully",
  "data": { ... }
}
```

**What happens internally:**
1. Acquires `graph_db_lock`
2. Validates edge exists via `has_edge()`
3. Deletes OLD relation embedding from `relationships_vdb` by hash ID
4. Merges `updated_data` into existing edge data
5. `upsert_edge()` to update graph DB
6. Recalculates embedding and upserts to `relationships_vdb`
7. `_edit_relation_done()` to persist

---

### 4.3 Delete Relation

**DELETE** `/documents/delete_relation`

Internal: `utils_graph.py:84` — `adelete_by_relation()`

```json
// Request
{
  "source_entity": "MyClass",
  "target_entity": "UserService"
}

// Response (200)
{
  "status": "success",
  "doc_id": "MyClass -> UserService",
  "message": "Successfully deleted relation from 'MyClass' to 'UserService'",
  "status_code": 200
}

// Not found (404)
{
  "status": "not_found",
  "doc_id": "MyClass -> UserService",
  "message": "Relation from 'MyClass' to 'UserService' does not exist",
  "status_code": 404
}
```

**What happens:**
1. Acquires `graph_db_lock`
2. Checks `has_edge()` — returns 404 if not
3. Computes hash IDs for both directions: `md5hash(src+tgt, "rel-")` and `md5hash(tgt+src, "rel-")`
4. Deletes from `relationships_vdb` by those IDs
5. `chunk_entity_relation_graph.remove_edges([(src, tgt)])`
6. `_delete_relation_done()` to persist

---

## 5. Merge Entities

**POST** `/graph/entities/merge`

Internal: `utils_graph.py:686` — `amerge_entities()`

```json
// Request
{
  "entities_to_change": ["OldClass", "DeprecatedClass"],
  "entity_to_change_into": "MyClass"
}

// Response (200)
{
  "status": "success",
  "message": "Successfully merged 2 entities into 'MyClass'",
  "data": {
    "merged_entity": "MyClass",
    "deleted_entities": ["OldClass", "DeprecatedClass"],
    "relationships_transferred": 15
  }
}
```

---

## 6. Graph Read Endpoints

### GET `/graphs?label=*&max_depth=3&max_nodes=1000`

Returns connected subgraph rooted at nodes whose label includes the parameter.

### GET `/graph/label/list`

Returns all entity/relation label types as a list.

### GET `/graph/label/popular?limit=300`

Returns most connected entity labels sorted by degree.

### GET `/graph/label/search?q=...&limit=50`

Fuzzy label search.

---

## 7. Storage Model

### Entity in graph DB

```json
{
  "entity_id": "MyClass",
  "entity_type": "CLASS",
  "description": "Handles user authentication",
  "source_id": "project/src/auth.py",
  "file_path": "project/src/auth.py",
  "created_at": 1719000000
}
```

### Entity in vector DB

| Field | Value |
|-------|-------|
| Key | `md5hash(entity_name, prefix="ent-")` |
| Embedding content | `"{entity_name}\n{description}"` |

### Relation in graph DB

```json
{
  "description": "MyClass depends on UserService",
  "keywords": "dependency, import",
  "source_id": "project/src/auth.py",
  "weight": 1.0,
  "file_path": "project/src/auth.py",
  "created_at": 1719000000
}
```

### Relation in vector DB

| Field | Value |
|-------|-------|
| Key | `md5hash(src + tgt, prefix="rel-")` |
| Embedding content | `"{keywords}\t{src}\n{tgt}\n{description}"` |

---

## 10. MCP Tool-to-API Endpoint Mapping

> External agents should call these HTTP endpoints directly at `http://localhost:9621`.
> The MCP SSE protocol (port 9622) is for opencode/stdio clients only.

| MCP Tool | HTTP Method | Endpoint | Parameters |
|----------|-------------|----------|------------|
| `check_entity_exists` | GET | `/graph/entity/exists?name=NAME` | `name` (query param) |
| `get_knowledge_graph` | GET | `/graphs?label=*&max_depth=3&max_nodes=1000` | `label`, `max_depth`, `max_nodes` |
| `get_graph_labels` | GET | `/graph/label/list` | None |
| `get_entity_details` | GET | `/graphs?label=*&max_depth=3&max_nodes=1000` (filter client-side) | Filter by `entity_name` in response |
| `get_entity_neighbors` | GET | `/graphs?label=*&max_depth=3&max_nodes=1000` (filter client-side) | Filter by `entity_name` in response |
| `query` | POST | `/query` | `{"query": "...", "mode": "hybrid", "only_need_context": false}` |
| `query_data` | POST | `/query/data` | `{"query": "...", "mode": "hybrid", "only_need_context": true}` |
| `get_codebase_stats` | GET | `/graph/label/list` + `/graphs?label=*&max_depth=3` (aggregate client-side) | None |
| `list_entities_by_type` | GET | `/graphs?label=*&max_depth=3&max_nodes=1000` (filter client-side) | Filter by `entity_type` in response |
| `search_entities` | GET | `/graphs?label=*&max_depth=3&max_nodes=1000` (regex filter client-side) | Filter by `pattern` in response |
| `get_entities_in_file` | GET | `/graphs?label=*&max_depth=3&max_nodes=1000` (filter by `file_path`) | Filter by `file_path` in response |
| `get_related_files` | POST | `/query/data` | `{"query": "...", "mode": "hybrid", "only_need_context": true}` |
| `get_relation_detail` | GET | `/graphs?label=*&max_depth=3&max_nodes=1000` (filter by `keywords`) | Filter by `entity_name` and `relation_type` in response |
| `get_method_signature` | GET | `/graphs?label=*&max_depth=3&max_nodes=1000` (search by name, parse description) | Find node matching `entity_name`, extract params from description |
| `find_related_files` | GET | `/graphs?label=*&max_depth=3&max_nodes=1000` (match by `file_path` or entity name) | Filter by `entity_or_file` in response |
| `get_interface_methods` | GET | `/graphs?label=*&max_depth=3&max_nodes=1000` (find interface→method edges, class→implements edges) | Filter by `interface_name` in response |
| `add_note_to_entity` | POST | `/graph/entity/create` + `/graph/relation/create` | Creates note entity and HAS_NOTE relation |

### Known API Issues

| Issue | Affected Endpoints | Workaround |
|-------|-------------------|------------|
| POST `/query` and `/query/data` time out | `query`, `query_data`, `get_related_files`, `find_related_files` | Ollama not reachable from Docker container (`host.docker.internal` may not resolve) |
| GET `/graphs` returns all nodes — no server-side filtering | `list_entities_by_type`, `search_entities`, `get_entities_in_file` | Filter client-side from response |
| Windows path backslash vs forward slash | `get_entities_in_file` | Normalize paths: replace `\` with `/` before comparing |
| `get_codebase_stats` HTTP 500 | `get_codebase_stats` | Call `get_graph_labels` + `get_knowledge_graph` separately and aggregate manually |

---

## 9. Graph DB Lock

Every CRUD operation uses `graph_db_lock` from `lightrag.kg.shared_storage`:

```python
graph_db_lock = get_graph_db_lock(enable_logging=False)
async with graph_db_lock:
    # Read/write graph DB
    # Read/write vector DB
    # index_done_callback() on all storage instances to persist
```

The lock ensures consistency across concurrent watchdog + MCP writes. The `index_done_callback()` pattern flushes changes through whatever storage backend is in use (Redis, file-based, etc.).
