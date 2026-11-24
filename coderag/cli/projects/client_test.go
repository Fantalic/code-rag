package projects

import (
	"io"
	"net/http"
	"strings"
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
	c := New("http://coderag.invalid")
	c.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&hits, 1)
		sawPath = req.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"project":{"id":"p1"}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	if err := c.Push(registry.Project{ID: "p1", Name: "demo"}, true); err != nil {
		t.Fatalf("push: %v", err)
	}
	if err := c.FocusPOST("p1"); err != nil {
		t.Fatalf("focus: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expected 2 hits, got %d", got)
	}
	if sawPath != "/coderag/projects/p1/focus" {
		t.Fatalf("unexpected last path %q", sawPath)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
