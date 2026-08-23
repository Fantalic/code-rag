package projects

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/coderag/registry"
)

func TestClientDisabledWhenBaseURLEmpty(t *testing.T) {
	c := New("")
	if c.enabled {
		t.Fatalf("expected disabled client")
	}
	if err := c.Push(registry.Project{ID: "x"}, true); err != nil {
		t.Fatalf("disabled client should swallow errors: %v", err)
	}
}

func TestPushAndFocusHitTheServer(t *testing.T) {
	var hits int32
	var sawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		sawPath = r.URL.Path
		if r.Body != nil {
			defer r.Body.Close()
			_ = json.NewDecoder(r.Body).Decode(&map[string]any{})
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"project":{"id":"p1"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.Push(registry.Project{ID: "p1", Name: "demo"}, true); err != nil {
		t.Fatalf("push: %v", err)
	}
	if err := c.FocusPOST("p1"); err != nil {
		t.Fatalf("focus: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expected 2 hits, got %d", got)
	}
	if sawPath == "" {
		t.Fatalf("server never saw any path")
	}
}
