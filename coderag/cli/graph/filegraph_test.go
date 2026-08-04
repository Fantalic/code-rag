package graph

import (
	"reflect"
	"testing"
)

func node(id, fp string) map[string]any {
	return map[string]any{
		"id": id,
		"properties": map[string]any{
			"entity_id": id,
			"file_path": fp,
		},
	}
}

func edge(src, tgt string) map[string]any {
	return map[string]any{
		"source": src,
		"target": tgt,
	}
}

func edgeSwapped(src, tgt string) map[string]any {
	return map[string]any{
		"source": tgt,
		"target": src,
		"properties": map[string]any{
			"source": src,
			"target": tgt,
		},
	}
}

func TestBuildRefsAndDependents(t *testing.T) {
	nodes := []map[string]any{
		node("A.X", "/p/a.cs"),
		node("B.Y", "/p/b.cs"),
		node("A.Y", "/p/a.cs"),
	}
	edges := []map[string]any{
		edge("A.X", "B.Y"),
	}
	g := Build(nodes, edges)

	if got, want := g.Refs("/p/a.cs"), []string{"/p/b.cs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Refs = %v, want %v", got, want)
	}
	if got, want := g.Dependents("/p/b.cs"), []string{"/p/a.cs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependents = %v, want %v", got, want)
	}
	if got := g.Entities("/p/a.cs"); !reflect.DeepEqual(got, []string{"A.X", "A.Y"}) {
		t.Errorf("Entities = %v", got)
	}
}

func TestBuildDirectionFromProperties(t *testing.T) {
	nodes := []map[string]any{
		node("A.X", "/p/a.cs"),
		node("B.Y", "/p/b.cs"),
	}
	// LightRAG read-back swaps source/target lexicographically; the true
	// direction lives in properties.source/target.
	edges := []map[string]any{
		edgeSwapped("A.X", "B.Y"),
	}
	g := Build(nodes, edges)

	if got, want := g.Refs("/p/a.cs"), []string{"/p/b.cs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Refs = %v, want %v (true direction)", got, want)
	}
	if got := g.Refs("/p/b.cs"); len(got) != 0 {
		t.Errorf("unexpected refs for b.cs: %v", got)
	}
}

func TestBuildSkipsSameFileAndDangling(t *testing.T) {
	nodes := []map[string]any{
		node("A.X", "/p/a.cs"),
		node("A.Y", "/p/a.cs"),
		node("B.Y", "/p/b.cs"),
	}
	edges := []map[string]any{
		edge("A.X", "A.Y"),   // same file -> skipped
		edge("A.X", "GONE"),  // dangling target -> skipped
		edge("B.Y", "A.X"),   // valid cross-file
	}
	g := Build(nodes, edges)

	if got := g.Refs("/p/b.cs"); !reflect.DeepEqual(got, []string{"/p/a.cs"}) {
		t.Errorf("Refs = %v, want [/p/a.cs]", got)
	}
	if got := g.Refs("/p/a.cs"); len(got) != 0 {
		t.Errorf("unexpected refs for a.cs: %v", got)
	}
}

func TestBuildCycles(t *testing.T) {
	nodes := []map[string]any{
		node("A.X", "/p/a.cs"),
		node("B.X", "/p/b.cs"),
		node("C.X", "/p/c.cs"),
	}
	edges := []map[string]any{
		edge("A.X", "B.X"),
		edge("B.X", "C.X"),
		edge("C.X", "A.X"),
	}
	g := Build(nodes, edges)

	cycles := g.Cycles()
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %v", cycles)
	}
	if got, want := cycles[0], []string{"/p/a.cs", "/p/b.cs", "/p/c.cs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("cycle = %v, want %v", got, want)
	}
}

func TestBuildNoCycles(t *testing.T) {
	nodes := []map[string]any{
		node("A.X", "/p/a.cs"),
		node("B.X", "/p/b.cs"),
	}
	edges := []map[string]any{
		edge("A.X", "B.X"),
	}
	g := Build(nodes, edges)
	if cycles := g.Cycles(); len(cycles) != 0 {
		t.Errorf("expected no cycles, got %v", cycles)
	}
}

func TestStats(t *testing.T) {
	nodes := []map[string]any{
		node("A.X", "/p/a.cs"),
		node("A.Y", "/p/a.cs"),
		node("B.Y", "/p/b.cs"),
	}
	edges := []map[string]any{
		edge("A.X", "B.Y"),
		edge("A.Y", "B.Y"),
	}
	g := Build(nodes, edges)
	stats := g.Stats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 files, got %d", len(stats))
	}
	byPath := map[string]FileStat{}
	for _, s := range stats {
		byPath[s.FilePath] = s
	}
	a := byPath["/p/a.cs"]
	if a.Nodes != 2 || a.FanIn != 0 || a.FanOut != 1 {
		t.Errorf("a.cs stats = %+v", a)
	}
	b := byPath["/p/b.cs"]
	if b.Nodes != 1 || b.FanIn != 1 || b.FanOut != 0 {
		t.Errorf("b.cs stats = %+v", b)
	}
}

func TestResolveFile(t *testing.T) {
	nodes := []map[string]any{
		node("A.X", "/p/src/a.cs"),
	}
	g := Build(nodes, []map[string]any{})

	if f, ok := g.ResolveFile("/p/src/a.cs"); !ok || f != "/p/src/a.cs" {
		t.Errorf("exact match failed: %q %v", f, ok)
	}
	if f, ok := g.ResolveFile("a.cs"); !ok || f != "/p/src/a.cs" {
		t.Errorf("basename match failed: %q %v", f, ok)
	}
	if f, ok := g.ResolveFile("src/a"); !ok || f != "/p/src/a.cs" {
		t.Errorf("substring match failed: %q %v", f, ok)
	}
	if _, ok := g.ResolveFile("nope"); ok {
		t.Errorf("expected no match for unknown file")
	}
}
