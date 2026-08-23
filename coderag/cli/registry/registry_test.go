package registry

import (
	"path/filepath"
	"sync"
	"testing"
)

func newTempRegistry(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()
	return New(filepath.Join(dir, "projects.json"))
}

func TestUpsertCreatesAndPersists(t *testing.T) {
	r := newTempRegistry(t)
	p, err := r.Upsert(Project{Path: "/tmp/foo", Kind: "csharp"}, nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if p.ID == "" || p.Name != "foo" || p.CreatedAt == 0 {
		t.Fatalf("unexpected project: %+v", p)
	}

	// Reload from disk
	r2 := New(r.filePath)
	list := r2.List()
	if len(list) != 1 || list[0].ID != p.ID {
		t.Fatalf("reload failed: %+v", list)
	}
}

func TestUpsertAccumulatesTotals(t *testing.T) {
	r := newTempRegistry(t)
	p, _ := r.Upsert(Project{Path: "/tmp/foo"}, nil)
	r.Upsert(Project{ID: p.ID, Path: p.Path}, map[string]int{"files": 3, "entities_created": 5})
	r.Upsert(Project{ID: p.ID, Path: p.Path}, map[string]int{"entities_created": 2})

	got, _ := r.Get(p.ID)
	if got.TotalCounts["files"] != 3 || got.TotalCounts["entities_created"] != 7 {
		t.Fatalf("totals wrong: %+v", got.TotalCounts)
	}
	if got.ScanCount != 2 {
		t.Fatalf("scancount want 2 got %d", got.ScanCount)
	}
}

func TestFocusSetAndForget(t *testing.T) {
	r := newTempRegistry(t)
	p, _ := r.Upsert(Project{Path: "/tmp/bar"}, nil)
	if err := r.SetFocused(p.ID); err != nil {
		t.Fatalf("focus: %v", err)
	}
	foc, ok := r.Focused()
	if !ok || foc.ID != p.ID {
		t.Fatalf("focus wrong: %+v ok=%v", foc, ok)
	}
	if err := r.Forget(p.ID); err != nil {
		t.Fatalf("forget: %v", err)
	}
	foc, ok = r.Focused()
	if ok || foc.ID != "" {
		t.Fatalf("focus should be cleared: %+v ok=%v", foc, ok)
	}
	if _, ok := r.Get(p.ID); ok {
		t.Fatalf("project should be gone")
	}
}

func TestFocusRejectsUnknown(t *testing.T) {
	r := newTempRegistry(t)
	if err := r.SetFocused("unknown"); err == nil {
		t.Fatalf("expected error on unknown id")
	}
	// Empty id is allowed (clears focus)
	if err := r.SetFocused(""); err != nil {
		t.Fatalf("clear focus should be a no-op: %v", err)
	}
}

func TestNewIDIsStable(t *testing.T) {
	id1 := NewID(`E:\_dev\foo\bar`)
	id2 := NewID(`E:/_dev/foo/bar`)
	id3 := NewID(`e:\_dev\foo\bar`)
	if id1 != id2 || id1 != id3 || len(id1) != 12 {
		t.Fatalf("ids should match across casing and slash style: %q %q %q", id1, id2, id3)
	}
}

func TestConcurrentUpsert(t *testing.T) {
	r := newTempRegistry(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := Project{Path: "/tmp/concurrent"}
			delta := map[string]int{"files": i + 1}
			_, _ = r.Upsert(p, delta)
		}(i)
	}
	wg.Wait()
	list := r.List()
	if len(list) != 1 {
		t.Fatalf("want 1 project got %d", len(list))
	}
	wantSum := 0
	for i := 0; i < 20; i++ {
		wantSum += i + 1
	}
	if list[0].TotalCounts["files"] != wantSum {
		t.Fatalf("totals want %d got %d", wantSum, list[0].TotalCounts["files"])
	}
}
