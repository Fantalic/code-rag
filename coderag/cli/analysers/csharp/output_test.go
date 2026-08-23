package csharpanalyser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewOutputConfig(t *testing.T) {
	oc := NewOutputConfig("out", "sln")
	if oc.OutputDir != "out" {
		t.Errorf("OutputDir = %q, want %q", oc.OutputDir, "out")
	}
	if oc.SlnDir != "sln" {
		t.Errorf("SlnDir = %q, want %q", oc.SlnDir, "sln")
	}
}

func TestWriteFileResult(t *testing.T) {
	dir := t.TempDir()
	oc := NewOutputConfig(dir, dir)

	fr := &FileResult{
		FilePath: filepath.Join(dir, "test.cs"),
		Entities: []Entity{
			{Name: "MyClass", Type: EntityClass, FilePath: "test.cs", LineNo: 10, Description: "Test class"},
		},
		Relations: []Relation{
			{Source: "A", Target: "B", Description: "calls", Keywords: "call", Weight: 0.5},
		},
		Exports: []string{"MyClass"},
		Imports: []string{"System"},
	}

	outPath, err := oc.WriteFileResult(fr)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "# EXPORTS: MyClass") {
		t.Errorf("missing EXPORTS header, got:\n%s", content)
	}
	if !strings.Contains(content, "# IMPORTS: System") {
		t.Errorf("missing IMPORTS header, got:\n%s", content)
	}
	if !strings.Contains(content, "CLASS|MyClass|test.cs|10|10|Test class") {
		t.Errorf("missing entity line, got:\n%s", content)
	}
	if !strings.Contains(content, "# FORMAT: 2") {
		t.Errorf("missing FORMAT:2 header, got:\n%s", content)
	}
	if !strings.Contains(content, "RELATION|A|B|calls|call|") {
		t.Errorf("missing relation line, got:\n%s", content)
	}
}

func TestWriteFileResultEmpty(t *testing.T) {
	dir := t.TempDir()
	oc := NewOutputConfig(dir, dir)

	fr := &FileResult{
		FilePath: filepath.Join(dir, "empty.cs"),
	}

	outPath, err := oc.WriteFileResult(fr)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "# EXPORTS:") {
		t.Errorf("missing EXPORTS header, got:\n%s", content)
	}
	if !strings.Contains(content, "# IMPORTS:") {
		t.Errorf("missing IMPORTS header, got:\n%s", content)
	}
}

func TestComputeDiff(t *testing.T) {
	dr := ComputeDiff([]string{"a", "b"}, []string{"a", "b"})
	if !dr.IsEmpty() {
		t.Error("expected empty diff for equal lines")
	}

	dr = ComputeDiff([]string{"a", "b"}, []string{"a"})
	if len(dr.RemovedSymbols) != 1 || dr.RemovedSymbols[0] != "b" {
		t.Errorf("RemovedSymbols = %v, want [b]", dr.RemovedSymbols)
	}

	dr = ComputeDiff([]string{"a"}, []string{"a", "b"})
	if len(dr.AddedSymbols) != 1 || dr.AddedSymbols[0] != "b" {
		t.Errorf("AddedSymbols = %v, want [b]", dr.AddedSymbols)
	}

	dr = ComputeDiff([]string{"a", "b"}, []string{"a", "c"})
	if len(dr.RemovedSymbols) != 1 || dr.RemovedSymbols[0] != "b" {
		t.Errorf("RemovedSymbols = %v, want [b]", dr.RemovedSymbols)
	}
	if len(dr.AddedSymbols) != 1 || dr.AddedSymbols[0] != "c" {
		t.Errorf("AddedSymbols = %v, want [c]", dr.AddedSymbols)
	}
}

func TestComputeDiffEmpty(t *testing.T) {
	dr := ComputeDiff(nil, nil)
	if !dr.IsEmpty() {
		t.Error("expected empty diff")
	}
}

func TestDiffReportIsEmpty(t *testing.T) {
	dr := &DiffReport{}
	if !dr.IsEmpty() {
		t.Error("expected empty")
	}
	dr.AddedSymbols = []string{"x"}
	if dr.IsEmpty() {
		t.Error("expected non-empty")
	}
}

func TestDiffReportString(t *testing.T) {
	dr := &DiffReport{
		RemovedSymbols: []string{"oldFunc"},
		AddedSymbols:   []string{"newFunc"},
	}
	s := dr.String()
	if !strings.Contains(s, "REMOVED_SYMBOL: oldFunc") {
		t.Errorf("missing REMOVED_SYMBOL, got:\n%s", s)
	}
	if !strings.Contains(s, "ADDED_SYMBOL:   newFunc") {
		t.Errorf("missing ADDED_SYMBOL, got:\n%s", s)
	}
}

func TestReadOutputExports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	err := os.WriteFile(path, []byte("# EXPORTS: A, B, C\n# IMPORTS: X\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	exports, err := ReadOutputExports(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(exports) != 3 || exports[0] != "A" || exports[1] != "B" || exports[2] != "C" {
		t.Errorf("got %v, want [A B C]", exports)
	}
}

func TestReadOutputExportsNoExports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	err := os.WriteFile(path, []byte("# IMPORTS: X\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	exports, err := ReadOutputExports(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(exports) != 0 {
		t.Errorf("got %v, want empty", exports)
	}
}

func TestWriteFullIndex(t *testing.T) {
	dir := t.TempDir()
	oc := NewOutputConfig(dir, "")

	index := NewGlobalIndex()
	index.Classes["MyClass"] = &ClassInfo{
		Name: "MyClass", FilePath: "test.cs", LineNo: 10,
		BaseType: "BaseClass", Interfaces: []string{"IDisposable"},
	}
	index.Classes["SimpleClass"] = &ClassInfo{
		Name: "SimpleClass", FilePath: "simple.cs", LineNo: 5,
	}
	index.Methods["MyMethod"] = []*MethodInfo{
		{Name: "MyMethod", FilePath: "test.cs", LineNo: 20, ReturnType: "void"},
	}
	index.Properties["MyProp"] = []*PropertyInfo{
		{Name: "MyProp", FilePath: "test.cs", LineNo: 30, Type: "int"},
	}

	files := []string{"test.cs", "simple.cs"}
	outPath, err := oc.WriteFullIndex(index, files)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "# GLOBAL INDEX") {
		t.Errorf("missing GLOBAL INDEX header, got:\n%s", content)
	}
	if !strings.Contains(content, "# Total files: 2") {
		t.Errorf("wrong file count, got:\n%s", content)
	}
	if !strings.Contains(content, "MyClass|test.cs|10 : BaseClass : IDisposable") {
		t.Errorf("missing class entry with base/interfaces, got:\n%s", content)
	}
	if !strings.Contains(content, "SimpleClass|simple.cs|5") {
		t.Errorf("missing simple class entry, got:\n%s", content)
	}
	if !strings.Contains(content, "MyMethod|test.cs|20|void") {
		t.Errorf("missing method entry, got:\n%s", content)
	}
	if !strings.Contains(content, "MyProp|test.cs|30|int") {
		t.Errorf("missing property entry, got:\n%s", content)
	}
}

func TestWriteCoverageReport(t *testing.T) {
	dir := t.TempDir()
	slnDir := dir
	projDir := filepath.Join(dir, "src", "App")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	oc := NewOutputConfig(filepath.Join(dir, ".lightrag"), slnDir)

	report := CoverageReport{Projects: []ProjectCoverage{
		{
			ProjectPath: filepath.Join(projDir, "App.csproj"),
			SourceFiles: []string{filepath.Join(projDir, "App.cs")},
			ProjectRefs: []string{},
		},
		{
			ProjectPath: filepath.Join(projDir, "Broken", "Broken.csproj"),
			Warnings:    []string{"parse_error: missing closing tag"},
		},
	}}
	outPath, err := oc.WriteCoverageReport(report)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "# COVERAGE REPORT") {
		t.Errorf("missing COVERAGE REPORT header, got:\n%s", content)
	}
	if !strings.Contains(content, `COVERAGE|[project]/src/App/App|{"files":1,"refs":0,"warnings":"-"}`) {
		t.Errorf("missing App COVERAGE row, got:\n%s", content)
	}
	if !strings.Contains(content, `COVERAGE|[project]/src/App/Broken/Broken|{"files":0,"refs":0,"warnings":"parse_error: missing closing tag"}`) {
		t.Errorf("missing Broken COVERAGE row, got:\n%s", content)
	}
}

func TestChangelogRows(t *testing.T) {
	dr := &DiffReport{
		AddedSymbols:   []string{"App.Service.Run"},
		RemovedSymbols: []string{"App.Service.Legacy"},
	}
	rows := ChangelogRows("src/App/Service.cs", dr, 1700000000)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d: %v", len(rows), rows)
	}
	if !strings.Contains(rows[0], `CHANGELOG|src/App/Service.cs|`) {
		t.Errorf("row[0] should start with CHANGELOG|file|, got %q", rows[0])
	}
	if !strings.Contains(rows[0], `"action":"added"`) || !strings.Contains(rows[0], `"symbol":"App.Service.Run"`) {
		t.Errorf("row[0] should describe added symbol, got %q", rows[0])
	}
	if !strings.Contains(rows[0], `"changed_at":1700000000`) {
		t.Errorf("row[0] should carry timestamp, got %q", rows[0])
	}
	if !strings.Contains(rows[1], `"action":"removed"`) || !strings.Contains(rows[1], `"symbol":"App.Service.Legacy"`) {
		t.Errorf("row[1] should describe removed symbol, got %q", rows[1])
	}
}

func TestWriteChangeLog(t *testing.T) {
	dir := t.TempDir()
	oc := NewOutputConfig(dir, dir)
	dr := &DiffReport{AddedSymbols: []string{"App.New"}}
	if _, err := oc.WriteChangeLog("src/App.cs", dr, 1700000000); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "_changelog.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "CHANGELOG|src/App.cs|") {
		t.Errorf("missing CHANGELOG row, got:\n%s", string(data))
	}
	// Empty diff must not touch the sidecar (keeps file stable).
	if _, err := oc.WriteChangeLog("src/Other.cs", &DiffReport{}, 0); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(path)
	if len(data2) != len(data) {
		t.Errorf("empty diff should not append, sizes %d vs %d", len(data), len(data2))
	}
}
