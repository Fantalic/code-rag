package csharpanalyser

import (
	"strings"
	"testing"
)

func TestNewScopeTracker(t *testing.T) {
	idx := NewGlobalIndex()
	st := NewScopeTracker(idx)
	if st == nil {
		t.Fatal("expected non-nil ScopeTracker")
	}
}

func TestAddFileResult(t *testing.T) {
	idx := NewGlobalIndex()
	st := NewScopeTracker(idx)

	fr := &FileResult{
		FilePath: "source.cs",
		Calls: []CallSite{
			{CallerFile: "source.cs", CallerMethod: "Caller", CalleeName: "Target", CalleeClass: "", LineNo: 5},
		},
	}
	st.AddFileResult("source.cs", fr)

	rels := st.ResolveCalls("source.cs")
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rels))
	}
	if rels[0].Source != "Caller" {
		t.Errorf("Source = %q, want %q", rels[0].Source, "Caller")
	}
	if rels[0].Target != "Target" {
		t.Errorf("Target = %q, want %q", rels[0].Target, "Target")
	}
}

func TestResolveCalls(t *testing.T) {
	index := NewGlobalIndex()
	index.Classes["TargetClass"] = &ClassInfo{
		Name: "TargetClass", FilePath: "target.cs",
	}
	index.Methods["TargetMethod"] = []*MethodInfo{{
		Name: "TargetMethod", ClassName: "TargetClass",
		FilePath: "target.cs",
	}}

	st := NewScopeTracker(index)
	fr := &FileResult{
		FilePath: "source.cs",
		Calls: []CallSite{
			{
				CallerFile: "source.cs", CallerMethod: "CallerMethod",
				CalleeName: "TargetMethod", CalleeClass: "TargetClass",
				LineNo: 10,
			},
		},
	}
	st.AddFileResult("source.cs", fr)

	rels := st.ResolveCalls("source.cs")
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rels))
	}

	rel := rels[0]
	if rel.Source != "CallerMethod" {
		t.Errorf("Source = %q, want %q", rel.Source, "CallerMethod")
	}
	if rel.Target != "TargetClass.TargetMethod" {
		t.Errorf("Target = %q, want %q", rel.Target, "TargetClass.TargetMethod")
	}
	if rel.Keywords != "call" {
		t.Errorf("Keywords = %q, want %q", rel.Keywords, "call")
	}
	if rel.Weight != 0.5 {
		t.Errorf("Weight = %f, want %f", rel.Weight, 0.5)
	}
	if !strings.Contains(rel.Description, "CallerMethod calls") {
		t.Errorf("Description = %q, want to contain 'CallerMethod calls'", rel.Description)
	}
}

func TestResolveCallsUnresolved(t *testing.T) {
	index := NewGlobalIndex()
	st := NewScopeTracker(index)

	fr := &FileResult{
		FilePath: "source.cs",
		Calls: []CallSite{
			{
				CallerFile: "source.cs", CallerMethod: "CallerMethod",
				CalleeName: "UnknownMethod", CalleeClass: "UnknownClass",
				LineNo: 10,
			},
		},
	}
	st.AddFileResult("source.cs", fr)

	rels := st.ResolveCalls("source.cs")
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rels))
	}
	if rels[0].Target != "UnknownClass.UnknownMethod" {
		t.Errorf("Target = %q, want %q", rels[0].Target, "UnknownClass.UnknownMethod")
	}
	if rels[0].Keywords != "call" {
		t.Errorf("Keywords = %q, want %q", rels[0].Keywords, "call")
	}
}

func TestResolveAllCalls(t *testing.T) {
	index := NewGlobalIndex()
	index.Classes["TargetClass"] = &ClassInfo{
		Name: "TargetClass", FilePath: "target.cs",
	}
	index.Methods["TargetMethod"] = []*MethodInfo{{
		Name: "TargetMethod", ClassName: "TargetClass",
		FilePath: "target.cs",
	}}

	st := NewScopeTracker(index)
	st.AddFileResult("source1.cs", &FileResult{
		FilePath: "source1.cs",
		Calls: []CallSite{
			{CallerFile: "source1.cs", CallerMethod: "M1", CalleeName: "TargetMethod", CalleeClass: "TargetClass", LineNo: 5},
		},
	})
	st.AddFileResult("source2.cs", &FileResult{
		FilePath: "source2.cs",
		Calls: []CallSite{
			{CallerFile: "source2.cs", CallerMethod: "M2", CalleeName: "TargetMethod", CalleeClass: "TargetClass", LineNo: 15},
		},
	})

	results := st.ResolveAllCalls()
	if len(results) != 2 {
		t.Fatalf("expected 2 files with relations, got %d", len(results))
	}
	if len(results["source1.cs"]) != 1 {
		t.Errorf("expected 1 relation for source1.cs, got %d", len(results["source1.cs"]))
	}
	if len(results["source2.cs"]) != 1 {
		t.Errorf("expected 1 relation for source2.cs, got %d", len(results["source2.cs"]))
	}

	r1 := results["source1.cs"][0]
	if r1.Source != "M1" {
		t.Errorf("source1.cs Source = %q, want %q", r1.Source, "M1")
	}
	r2 := results["source2.cs"][0]
	if r2.Source != "M2" {
		t.Errorf("source2.cs Source = %q, want %q", r2.Source, "M2")
	}
}
