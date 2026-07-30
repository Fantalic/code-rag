package crud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// testTransport is an in-memory RoundTripper that invokes a handler directly
// without touching the network stack (required on machines that block loopback TCP).
type testTransport struct {
	handler http.Handler
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	t.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

func okResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// testCfg creates a Config backed by the given handler using an in-memory
// transport. The BaseURL is ignored by the transport and only needs to be
// syntactically valid for http.NewRequest.
func testCfg(handler http.HandlerFunc) *Config {
	transport := &testTransport{handler: handler}
	return &Config{
		BaseURL:    "http://test.local",
		Timeout:    5 * time.Second,
		HTTPClient: &http.Client{Transport: transport},
	}
}

// ---------------------------------------------------------------------------
// NewConfig
// ---------------------------------------------------------------------------

func TestNewConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("LIGHTRAG_URL", "")
		t.Setenv("LIGHTRAG_TIMEOUT", "")
		t.Setenv("LIGHTRAG_API_KEY", "")
		c := NewConfig()
		if c.BaseURL != "http://localhost:9621" {
			t.Fatalf("expected default localhost:9621, got %s", c.BaseURL)
		}
		if c.Timeout != DefaultTimeout {
			t.Fatalf("expected DefaultTimeout %v, got %v", DefaultTimeout, c.Timeout)
		}
		if c.APIKey != "" {
			t.Fatalf("expected empty API key, got %s", c.APIKey)
		}
	})

	t.Run("env_override", func(t *testing.T) {
		t.Setenv("LIGHTRAG_URL", "http://example.com:9999")
		t.Setenv("LIGHTRAG_TIMEOUT", "10s")
		t.Setenv("LIGHTRAG_API_KEY", "secret-key")
		c := NewConfig()
		if c.BaseURL != "http://example.com:9999" {
			t.Fatalf("expected http://example.com:9999, got %s", c.BaseURL)
		}
		if c.Timeout != 10*time.Second {
			t.Fatalf("expected 10s, got %v", c.Timeout)
		}
		if c.APIKey != "secret-key" {
			t.Fatalf("expected secret-key, got %s", c.APIKey)
		}
	})

	t.Run("bad_timeout_fallsback", func(t *testing.T) {
		t.Setenv("LIGHTRAG_TIMEOUT", "not-a-duration")
		c := NewConfig()
		if c.Timeout != DefaultTimeout {
			t.Fatalf("expected DefaultTimeout %v, got %v", DefaultTimeout, c.Timeout)
		}
	})
}

// ---------------------------------------------------------------------------
// Headers
// ---------------------------------------------------------------------------

func TestHeaders(t *testing.T) {
	t.Run("without_key", func(t *testing.T) {
		c := &Config{BaseURL: "http://localhost:9621"}
		h := c.Headers()
		if h["Content-Type"] != "application/json" {
			t.Fatalf("expected application/json, got %s", h["Content-Type"])
		}
		if _, ok := h["X-API-Key"]; ok {
			t.Fatal("unexpected X-API-Key header")
		}
	})

	t.Run("with_key", func(t *testing.T) {
		c := &Config{BaseURL: "http://localhost:9621", APIKey: "my-api-key"}
		h := c.Headers()
		if h["Content-Type"] != "application/json" {
			t.Fatalf("expected application/json, got %s", h["Content-Type"])
		}
		if h["X-API-Key"] != "my-api-key" {
			t.Fatalf("expected my-api-key, got %s", h["X-API-Key"])
		}
	})
}

// ---------------------------------------------------------------------------
// CreateEntity
// ---------------------------------------------------------------------------

func TestCreateEntity(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var (
			mu         sync.Mutex
			method, pt string
		)
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			method = r.Method
			pt = r.URL.Path
			mu.Unlock()
			okResponse(w, map[string]any{"status": "ok"})
		}))

		res, err := c.CreateEntity("TestEntity", "A test", "concept", "src1", "/path/file.go")
		if err != nil {
			t.Fatal(err)
		}
		if res["status"] != "ok" {
			t.Fatalf("expected ok, got %v", res["status"])
		}
		mu.Lock()
		if method != http.MethodPost {
			t.Errorf("expected POST, got %s", method)
		}
		if pt != "/graph/entity/create" {
			t.Errorf("unexpected path: %s", pt)
		}
		mu.Unlock()
	})

	t.Run("error", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"bad request"}`))
		}))

		_, err := c.CreateEntity("X", "desc", "type", "s", "f")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "HTTP 400") {
			t.Fatalf("expected HTTP 400 error, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// UpdateEntity
// ---------------------------------------------------------------------------

func TestUpdateEntity(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var (
			mu         sync.Mutex
			method, pt string
		)
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			method = r.Method
			pt = r.URL.Path
			mu.Unlock()
			okResponse(w, map[string]any{"status": "updated"})
		}))

		res, err := c.UpdateEntity("E1", map[string]any{"description": "new desc"})
		if err != nil {
			t.Fatal(err)
		}
		if res["status"] != "updated" {
			t.Fatalf("expected updated, got %v", res["status"])
		}
		mu.Lock()
		if method != http.MethodPost {
			t.Errorf("expected POST, got %s", method)
		}
		if pt != "/graph/entity/edit" {
			t.Errorf("unexpected path: %s", pt)
		}
		mu.Unlock()
	})

	t.Run("not_found", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}))

		_, err := c.UpdateEntity("Missing", map[string]any{"x": "y"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "HTTP 404") {
			t.Fatalf("expected HTTP 404, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// DeleteEntity
// ---------------------------------------------------------------------------

func TestDeleteEntity(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var (
			mu         sync.Mutex
			method, pt string
		)
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			method = r.Method
			pt = r.URL.Path
			mu.Unlock()
			okResponse(w, map[string]any{"status": "deleted"})
		}))

		res, err := c.DeleteEntity("E1")
		if err != nil {
			t.Fatal(err)
		}
		if res["status"] != "deleted" {
			t.Fatalf("expected deleted, got %v", res["status"])
		}
		mu.Lock()
		if method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", method)
		}
		if pt != "/documents/delete_entity" {
			t.Errorf("unexpected path: %s", pt)
		}
		mu.Unlock()
	})

	t.Run("not_found", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}))

		_, err := c.DeleteEntity("Missing")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("server_error", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal error"}`))
		}))

		_, err := c.DeleteEntity("E2")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "HTTP 500") {
			t.Fatalf("expected HTTP 500, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// CheckEntityExists
// ---------------------------------------------------------------------------

func TestCheckEntityExists(t *testing.T) {
	t.Run("exists_true", func(t *testing.T) {
		var (
			mu             sync.Mutex
			method, pt, nm string
		)
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			method = r.Method
			pt = r.URL.Path
			nm = r.URL.Query().Get("name")
			mu.Unlock()
			okResponse(w, map[string]any{"exists": true})
		}))

		if !c.CheckEntityExists("MyEntity") {
			t.Fatal("expected true")
		}
		mu.Lock()
		if method != http.MethodGet {
			t.Errorf("expected GET, got %s", method)
		}
		if pt != "/graph/entity/exists" {
			t.Errorf("unexpected path: %s", pt)
		}
		if nm != "MyEntity" {
			t.Errorf("expected name=MyEntity, got %s", nm)
		}
		mu.Unlock()
	})

	t.Run("exists_false", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			okResponse(w, map[string]any{"exists": false})
		}))

		if c.CheckEntityExists("Missing") {
			t.Fatal("expected false")
		}
	})

	t.Run("error_status", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		if c.CheckEntityExists("X") {
			t.Fatal("expected false on server error")
		}
	})

	t.Run("bad_json_returns_false", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not json`))
		}))

		if c.CheckEntityExists("X") {
			t.Fatal("expected false on bad JSON")
		}
	})
}

// ---------------------------------------------------------------------------
// CreateRelation
// ---------------------------------------------------------------------------

func TestCreateRelation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var (
			mu         sync.Mutex
			method, pt string
		)
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			method = r.Method
			pt = r.URL.Path
			mu.Unlock()
			okResponse(w, map[string]any{"status": "relation_created"})
		}))

		res, err := c.CreateRelation("Src", "Tgt", "desc", "kw", 1.5, "sid", "fp")
		if err != nil {
			t.Fatal(err)
		}
		if res["status"] != "relation_created" {
			t.Fatalf("expected relation_created, got %v", res["status"])
		}
		mu.Lock()
		if method != http.MethodPost {
			t.Errorf("expected POST, got %s", method)
		}
		if pt != "/graph/relation/create" {
			t.Errorf("unexpected path: %s", pt)
		}
		mu.Unlock()
	})

	t.Run("bad_request", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid relation"}`))
		}))

		_, err := c.CreateRelation("", "", "", "", 0, "", "")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "HTTP 400") {
			t.Fatalf("expected HTTP 400, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// DeleteRelation
// ---------------------------------------------------------------------------

func TestDeleteRelation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var (
			mu         sync.Mutex
			method, pt string
		)
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			method = r.Method
			pt = r.URL.Path
			mu.Unlock()
			okResponse(w, map[string]any{"status": "relation_deleted"})
		}))

		res, err := c.DeleteRelation("Src", "Tgt")
		if err != nil {
			t.Fatal(err)
		}
		if res["status"] != "relation_deleted" {
			t.Fatalf("expected relation_deleted, got %v", res["status"])
		}
		mu.Lock()
		if method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", method)
		}
		if pt != "/documents/delete_relation" {
			t.Errorf("unexpected path: %s", pt)
		}
		mu.Unlock()
	})

	t.Run("not_found", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"relation not found"}`))
		}))

		_, err := c.DeleteRelation("Missing", "Tgt")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// ---------------------------------------------------------------------------
// GetAllEntities
// ---------------------------------------------------------------------------

func TestGetAllEntities(t *testing.T) {
	t.Run("with_nodes", func(t *testing.T) {
		var (
			mu                sync.Mutex
			method, pt, label string
		)
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			method = r.Method
			pt = r.URL.Path
			label = r.URL.Query().Get("label")
			mu.Unlock()
			okResponse(w, GraphResponse{
				Nodes: []map[string]any{
					{"id": "n1", "properties": map[string]any{"entity_name": "E1"}},
					{"id": "n2", "properties": map[string]any{"entity_name": "E2"}},
				},
			})
		}))

		nodes, err := c.GetAllEntities()
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(nodes))
		}
		mu.Lock()
		if method != http.MethodGet {
			t.Errorf("expected GET, got %s", method)
		}
		if pt != "/graphs" {
			t.Errorf("unexpected path: %s", pt)
		}
		if label != "*" {
			t.Errorf("expected label=*, got %s", label)
		}
		mu.Unlock()
	})

	t.Run("empty", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			okResponse(w, GraphResponse{Nodes: nil})
		}))

		nodes, err := c.GetAllEntities()
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != 0 {
			t.Fatalf("expected 0 nodes, got %d", len(nodes))
		}
	})

	t.Run("server_error", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))

		_, err := c.GetAllEntities()
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "HTTP 403") {
			t.Fatalf("expected HTTP 403, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// GetAllRelations
// ---------------------------------------------------------------------------

func TestGetAllRelations(t *testing.T) {
	t.Run("with_edges", func(t *testing.T) {
		var (
			mu         sync.Mutex
			method, pt string
			maxDepth   string
		)
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			method = r.Method
			pt = r.URL.Path
			maxDepth = r.URL.Query().Get("max_depth")
			mu.Unlock()
			okResponse(w, map[string]any{
				"edges": []map[string]any{
					{
						"source":     "A",
						"target":     "B",
						"properties": map[string]any{"keywords": "k1", "source_id": "s1", "file_path": "f1"},
					},
					{
						"source":     "C",
						"target":     "D",
						"properties": map[string]any{"keywords": "k2", "source_id": "s2", "file_path": "f2"},
					},
				},
			})
		}))

		edges, err := c.GetAllRelations()
		if err != nil {
			t.Fatal(err)
		}
		if len(edges) != 2 {
			t.Fatalf("expected 2 edges, got %d", len(edges))
		}
		if edges[0].Source != "A" || edges[0].Target != "B" {
			t.Fatalf("unexpected edge 0: %+v", edges[0])
		}
		if edges[0].Keywords != "k1" || edges[0].SourceID != "s1" || edges[0].FilePath != "f1" {
			t.Fatalf("unexpected edge 0 properties: %+v", edges[0])
		}
		mu.Lock()
		if method != http.MethodGet {
			t.Errorf("expected GET, got %s", method)
		}
		if pt != "/graphs" {
			t.Errorf("unexpected path: %s", pt)
		}
		if maxDepth != "2" {
			t.Errorf("expected max_depth=2, got %s", maxDepth)
		}
		mu.Unlock()
	})

	t.Run("skip_empty_source_target", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			okResponse(w, map[string]any{
				"edges": []map[string]any{
					{"source": "", "target": ""},
					{"source": "X", "target": "Y", "properties": map[string]any{"keywords": "k"}},
				},
			})
		}))

		edges, err := c.GetAllRelations()
		if err != nil {
			t.Fatal(err)
		}
		if len(edges) != 1 {
			t.Fatalf("expected 1 edge after skipping empty, got %d", len(edges))
		}
		if edges[0].Source != "X" || edges[0].Target != "Y" {
			t.Fatalf("unexpected edge: %+v", edges[0])
		}
	})

	t.Run("no_edges", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			okResponse(w, map[string]any{"edges": nil})
		}))

		edges, err := c.GetAllRelations()
		if err != nil {
			t.Fatal(err)
		}
		if len(edges) != 0 {
			t.Fatalf("expected 0 edges, got %d", len(edges))
		}
	})
}

// ---------------------------------------------------------------------------
// SetWorkspace
// ---------------------------------------------------------------------------

func TestSetWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var (
			mu         sync.Mutex
			method, pt string
		)
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			method = r.Method
			pt = r.URL.Path
			mu.Unlock()
			okResponse(w, map[string]any{"status": "ok"})
		}))

		err := c.SetWorkspace("my-workspace")
		if err != nil {
			t.Fatal(err)
		}
		if c.Workspace != "my-workspace" {
			t.Fatalf("expected workspace to be set to my-workspace, got %s", c.Workspace)
		}
		mu.Lock()
		if method != http.MethodPost {
			t.Errorf("expected POST, got %s", method)
		}
		if pt != "/workspace" {
			t.Errorf("unexpected path: %s", pt)
		}
		mu.Unlock()
	})

	t.Run("failure", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		err := c.SetWorkspace("should-fail")
		if err == nil {
			t.Fatal("expected error")
		}
		if c.Workspace != "" {
			t.Fatalf("expected workspace to remain empty, got %s", c.Workspace)
		}
	})
}

// ---------------------------------------------------------------------------
// ClearAll
// ---------------------------------------------------------------------------

func TestClearAll(t *testing.T) {
	var (
		mu          sync.Mutex
		methods, pt []string
		callCount   int
	)
	c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		methods = append(methods, r.Method)
		pt = append(pt, r.URL.Path)
		callCount++
		mu.Unlock()
		okResponse(w, map[string]any{"status": "success", "data": map[string]any{}})
	}))

	err := c.ClearAll()
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
	if methods[0] != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", methods[0])
	}
	if pt[0] != "/documents" {
		t.Errorf("unexpected path: %s", pt[0])
	}
	if methods[1] != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", methods[1])
	}
	if pt[1] != "/graph" {
		t.Errorf("unexpected path: %s", pt[1])
	}
	mu.Unlock()
}

// ---------------------------------------------------------------------------
// IngestText
// ---------------------------------------------------------------------------

func TestIngestText(t *testing.T) {
	var (
		mu             sync.Mutex
		method, pt     string
		bodyText       string
		bodyFileSource string
	)
	c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		method = r.Method
		pt = r.URL.Path
		if t, ok := body["text"].(string); ok {
			bodyText = t
		}
		if f, ok := body["file_source"].(string); ok {
			bodyFileSource = f
		}
		mu.Unlock()
		okResponse(w, map[string]any{"status": "ingested"})
	}))

	err := c.IngestText("hello world", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if method != http.MethodPost {
		t.Errorf("expected POST, got %s", method)
	}
	if pt != "/documents/text" {
		t.Errorf("unexpected path: %s", pt)
	}
	if bodyText != "hello world" {
		t.Errorf("unexpected text: %s", bodyText)
	}
	if bodyFileSource != "test.txt" {
		t.Errorf("unexpected file_source: %s", bodyFileSource)
	}
	mu.Unlock()
}

// ---------------------------------------------------------------------------
// DeleteDocument
// ---------------------------------------------------------------------------

func TestDeleteDocument(t *testing.T) {
	var (
		mu         sync.Mutex
		method, pt string
	)
	c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		method = r.Method
		pt = r.URL.Path
		mu.Unlock()
		okResponse(w, map[string]any{"status": "document_deleted"})
	}))

	err := c.DeleteDocument("doc.txt")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if method != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", method)
	}
	if pt != "/documents" {
		t.Errorf("unexpected path: %s", pt)
	}
	mu.Unlock()
}

// ---------------------------------------------------------------------------
// Query
// ---------------------------------------------------------------------------

func TestQuery(t *testing.T) {
	var (
		mu         sync.Mutex
		method, pt string
		queryText  string
	)
	c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		method = r.Method
		pt = r.URL.Path
		if txt, ok := body["text"].(string); ok {
			queryText = txt
		}
		mu.Unlock()
		okResponse(w, map[string]any{"answer": "42"})
	}))

	res, err := c.Query("what is this?")
	if err != nil {
		t.Fatal(err)
	}
	if res["answer"] != "42" {
		t.Fatalf("expected answer=42, got %v", res["answer"])
	}
	mu.Lock()
	if method != http.MethodPost {
		t.Errorf("expected POST, got %s", method)
	}
	if pt != "/query/text" {
		t.Errorf("unexpected path: %s", pt)
	}
	if queryText != "what is this?" {
		t.Errorf("unexpected text: %s", queryText)
	}
	mu.Unlock()
}

// ---------------------------------------------------------------------------
// GetEntitiesByFilePath
// ---------------------------------------------------------------------------

func TestGetEntitiesByFilePath(t *testing.T) {
	t.Run("matched", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			okResponse(w, GraphResponse{
				Nodes: []map[string]any{
					{"properties": map[string]any{"entity_name": "E1", "file_path": "/src/a.go"}},
					{"properties": map[string]any{"entity_name": "E2", "file_path": "/src/b.go"}},
					{"properties": map[string]any{"entity_name": "E3", "file_path": "/src/a.go"}},
				},
			})
		}))

		names, err := c.GetEntitiesByFilePath("/src/a.go")
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != 2 {
			t.Fatalf("expected 2 names, got %d: %v", len(names), names)
		}
		if names[0] != "E1" || names[1] != "E3" {
			t.Fatalf("unexpected names: %v", names)
		}
	})

	t.Run("unmatched", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			okResponse(w, GraphResponse{
				Nodes: []map[string]any{
					{"properties": map[string]any{"entity_name": "E1", "file_path": "/other.go"}},
				},
			})
		}))

		names, err := c.GetEntitiesByFilePath("/nonexistent.go")
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != 0 {
			t.Fatalf("expected 0 names, got %d", len(names))
		}
	})

	t.Run("skip_missing_properties", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			okResponse(w, GraphResponse{
				Nodes: []map[string]any{
					{"id": "n1"},
					{"properties": map[string]any{"entity_name": "E2", "file_path": "/t.go"}},
				},
			})
		}))

		names, err := c.GetEntitiesByFilePath("/t.go")
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != 1 || names[0] != "E2" {
			t.Fatalf("expected [E2], got %v", names)
		}
	})
}

// ---------------------------------------------------------------------------
// DeleteEntitiesByFilePath
// ---------------------------------------------------------------------------

func TestDeleteEntitiesByFilePath(t *testing.T) {
	t.Run("with_entities", func(t *testing.T) {
		var mu sync.Mutex
		var deleteCallCount int
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/graphs":
				okResponse(w, GraphResponse{
					Nodes: []map[string]any{
						{"properties": map[string]any{"entity_name": "E1", "file_path": "/del.go"}},
						{"properties": map[string]any{"entity_name": "E2", "file_path": "/del.go"}},
					},
				})
			case "/documents/delete_entity":
				mu.Lock()
				deleteCallCount++
				mu.Unlock()
				okResponse(w, map[string]any{"status": "deleted"})
			}
		}))

		err := c.DeleteEntitiesByFilePath("/del.go")
		if err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		if deleteCallCount != 2 {
			t.Errorf("expected 2 delete calls, got %d", deleteCallCount)
		}
		mu.Unlock()
	})

	t.Run("no_entities", func(t *testing.T) {
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			okResponse(w, GraphResponse{Nodes: nil})
		}))

		err := c.DeleteEntitiesByFilePath("/empty.go")
		if err != nil {
			t.Fatal(err)
		}
	})
}

// ---------------------------------------------------------------------------
// UpdateRelation
// ---------------------------------------------------------------------------

func TestUpdateRelation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var (
			mu         sync.Mutex
			method, pt string
		)
		c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			method = r.Method
			pt = r.URL.Path
			mu.Unlock()
			okResponse(w, map[string]any{"status": "updated"})
		}))

		res, err := c.UpdateRelation("src", "tgt", map[string]any{"weight": 2.0})
		if err != nil {
			t.Fatal(err)
		}
		if res["status"] != "updated" {
			t.Fatalf("expected updated, got %v", res["status"])
		}
		mu.Lock()
		if method != http.MethodPost {
			t.Errorf("expected POST, got %s", method)
		}
		if pt != "/graph/relation/edit" {
			t.Errorf("unexpected path: %s", pt)
		}
		mu.Unlock()
	})
}

// ---------------------------------------------------------------------------
// Edge-case tests
// ---------------------------------------------------------------------------

func TestCheckEntityExistsRequestFailure(t *testing.T) {
	c := &Config{
		BaseURL: "http://127.0.0.1:1",
		Timeout: time.Second,
	}
	if c.CheckEntityExists("X") {
		t.Fatal("expected false when server unreachable")
	}
}

func TestGetAllEntitiesRequestFailure(t *testing.T) {
	c := &Config{
		BaseURL: "http://127.0.0.1:1",
		Timeout: time.Second,
	}
	_, err := c.GetAllEntities()
	if err == nil {
		t.Fatal("expected error when server unreachable")
	}
}

func TestGetAllRelationsBadJSON(t *testing.T) {
	c := testCfg(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid`))
	}))

	_, err := c.GetAllRelations()
	if err == nil {
		t.Fatal("expected error on bad JSON")
	}
}

func TestRelationsEdgeStruct(t *testing.T) {
	e := RelationEdge{
		Source:   "a",
		Target:   "b",
		Keywords: "k",
		SourceID: "s",
		FilePath: "f",
	}
	if e.Source != "a" || e.Target != "b" || e.Keywords != "k" || e.SourceID != "s" || e.FilePath != "f" {
		t.Fatal("RelationEdge struct fields not set correctly")
	}
}

func TestPayloadMarshalError(t *testing.T) {
	c := &Config{BaseURL: "http://localhost:9621"}
	_, err := c.do(http.MethodPost, "/test", make(chan int))
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "marshal payload") {
		t.Fatalf("expected marshal payload error, got %v", err)
	}
}

func TestConfigDefaults(t *testing.T) {
	c := &Config{}
	if c.BaseURL != "" {
		t.Fatalf("expected empty BaseURL, got %s", c.BaseURL)
	}
	if c.Timeout != 0 {
		t.Fatalf("expected zero Timeout, got %v", c.Timeout)
	}
	if c.APIKey != "" {
		t.Fatalf("expected empty APIKey, got %s", c.APIKey)
	}
	if c.Workspace != "" {
		t.Fatalf("expected empty Workspace, got %s", c.Workspace)
	}
}
