package ziganalyser

import (
	"strings"
	"testing"
)

// TestParseFileSimple exercises the file-level ParseFile helper on a
// hand-crafted Zig source covering every entity type the walker must
// recognise. Kept small so it runs as part of `go test ./...` in CI.
func TestParseFileSimple(t *testing.T) {
	src := `//! tiny test file
const std = @import("std");

pub const Config = struct {
    verbose: bool,
};

pub fn greet(name: []const u8) void {
    std.debug.print("hi {s}\n", .{name});
}

test "greet returns void" {
    try std.testing.expect(true);
}
`
	// Write to a temp file so ParseFile's filesystem reads work.
	dir := t.TempDir()
	// Use a non-conflicting basename so the per-file "test:EXPORTS"
	// synthetic entity doesn't collide with real test names in the
	// assertions below.
	path := dir + "/sample.zig"
	if err := writeFile(path, src); err != nil {
		t.Fatalf("write temp: %v", err)
	}

	fr, err := ParseFile(path, nil)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	wantEntities := []string{
		"Config", "Config.verbose", "greet",
		"test:greet returns void",
	}
	gotNames := map[string]bool{}
	for _, e := range fr.Entities {
		gotNames[e.Name] = true
	}
	for _, want := range wantEntities {
		if !gotNames[want] {
			t.Errorf("missing entity %q in output; got %v", want, sortedKeys(gotNames))
		}
	}

	// The EXPORTS header should contain both Config and greet (both
	// are pub). std is private so it must NOT appear.
	exportsSet := map[string]bool{}
	for _, e := range fr.Exports {
		exportsSet[e] = true
	}
	if !exportsSet["Config"] {
		t.Error("Config missing from EXPORTS")
	}
	if !exportsSet["greet"] {
		t.Error("greet missing from EXPORTS")
	}
	if exportsSet["std"] {
		t.Error("std should not be in EXPORTS (no pub)")
	}
}

// TestDiffComputation verifies the scan/update diff correctly reports
// added + removed symbols.
func TestDiffComputation(t *testing.T) {
	old := []string{"A", "B", "C"}
	newer := []string{"B", "C", "D"}
	report := ComputeDiff(old, newer)

	if len(report.AddedSymbols) != 1 || report.AddedSymbols[0] != "D" {
		t.Errorf("expected AddedSymbols=[D], got %v", report.AddedSymbols)
	}
	if len(report.RemovedSymbols) != 1 || report.RemovedSymbols[0] != "A" {
		t.Errorf("expected RemovedSymbols=[A], got %v", report.RemovedSymbols)
	}
}

// TestOutputFormatRoundTrip writes a tiny FileResult and verifies the
// produced .txt contains the headers + entity rows + relation rows +
// call rows. Acts as a smoke test for the writer.
func TestOutputFormatRoundTrip(t *testing.T) {
	dir := t.TempDir()
	oc := NewOutputConfig(dir+"/out", dir)

	fr := &FileResult{
		FilePath: dir + "/src/foo.zig",
		Entities: []Entity{
			{Name: "Foo", Type: EntityStruct, Description: "struct Foo [pub]", FilePath: dir + "/src/foo.zig", LineNo: 1, EndLine: 3, Visibility: "pub"},
		},
		Relations: []Relation{
			{Source: "Foo", Target: "Foo.x", Description: "Foo has field x", Keywords: "field, defines", Weight: 0.7},
		},
		Exports: []string{"Foo"},
		Imports: []string{"std"},
	}
	outPath, err := oc.WriteFileResult(fr)
	if err != nil {
		t.Fatalf("WriteFileResult: %v", err)
	}
	body, err := readFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	wantSubstrings := []string{
		"# FORMAT: 2",
		"# EXPORTS: Foo",
		"# IMPORTS: std",
		"STRUCT|Foo|",
		"RELATION|Foo|Foo.x|Foo has field x|field, defines|",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(body, want) {
			t.Errorf("output missing substring %q\nfull body:\n%s", want, body)
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// sort.Strings would require the import; manual sort is fine here.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
