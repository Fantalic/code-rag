package status

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// waitFor polls cond until true or the timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}

func TestReporterPostsBatch(t *testing.T) {
	flushInterval = 20 * time.Millisecond

	var mu sync.Mutex
	var got [][]Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/coderag/status/events" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var batch []Event
		if err := json.Unmarshal(body, &batch); err != nil {
			t.Errorf("bad json: %v", err)
		}
		mu.Lock()
		got = append(got, batch)
		mu.Unlock()
	}))
	defer srv.Close()

	rep := New(srv.URL)
	rep.Stage("analyse", "analysing %d files", 3)
	rep.Counts("crud", map[string]int{"files": 3})
	rep.Close()

	mu.Lock()
	defer mu.Unlock()
	total := 0
	for _, b := range got {
		total += len(b)
	}
	if total != 2 {
		t.Fatalf("expected 2 events delivered, got %d across %d batches", total, len(got))
	}

	// Merge all events for field checks (order across flushes is stable here).
	var evs []Event
	for _, b := range got {
		evs = append(evs, b...)
	}
	first := evs[0]
	if first.RunID == "" {
		t.Error("run_id not set")
	}
	if first.Stage != "analyse" || first.Message != "analysing 3 files" || first.Level != "info" {
		t.Errorf("unexpected event fields: %+v", first)
	}
	if first.TS <= 0 {
		t.Error("timestamp not set")
	}
	last := evs[1]
	if last.Counts["files"] != 3 {
		t.Errorf("counts not carried: %+v", last)
	}
}

func TestReporterDisabledByEnv(t *testing.T) {
	flushInterval = 20 * time.Millisecond
	t.Setenv("CODERAG_STATUS_DISABLE", "1")

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	rep := New(srv.URL)
	rep.Stage("scan", "should not be sent")
	rep.Close()

	if called {
		t.Fatal("request was made although reporter is disabled")
	}
}

func TestReporterNilSafe(t *testing.T) {
	var rep *Reporter
	// All methods must be no-ops on a nil reporter (hooks call Default which
	// may legitimately be nil).
	rep.Stage("scan", "x")
	rep.Warn("scan", "x")
	rep.Error("scan", "x")
	rep.Detail("scan", "d", "x")
	rep.Counts("scan", map[string]int{"a": 1})
	rep.Done("x")
	rep.Close()
	if rep.RunID() != "" {
		t.Error("nil RunID should be empty")
	}
}

func TestReporterSurvivesDeadEndpoint(t *testing.T) {
	flushInterval = 20 * time.Millisecond

	// Port 1 on localhost -> connection refused immediately.
	rep := New("http://127.0.0.1:1")

	start := time.Now()
	for i := 0; i < 1000; i++ {
		rep.Stage("scan", "spam %d", i)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("emit blocked pipeline: %v", elapsed)
	}
	rep.Close() // must return despite failing posts
}

func TestSetDefaultRestore(t *testing.T) {
	restore := SetDefault(nil)
	if Default != nil {
		t.Fatal("expected Default to be nil after SetDefault(nil)")
	}
	restore()
	// After restore it must simply not panic whatever it was.
}
