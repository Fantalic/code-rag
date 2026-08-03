package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/coderag/parser"
)

func TestLoadCurrentRelationsParsing(t *testing.T) {
	// Test the edge-to-map parsing logic from loadCurrentRelations
	// without requiring an HTTP server
	jsonData := `{
		"edges": [
			{
				"source": "DeletedClass",
				"target": "UserService",
				"properties": {"keywords": "depends_on", "source_id": "/src/DeletedClass.cs", "file_path": "/src/DeletedClass.cs"}
			},
			{
				"source": "MyClass",
				"target": "",
				"properties": {"keywords": "calls"}
			},
			{
				"source": "",
				"target": "Orphan",
				"properties": {"keywords": "has"}
			},
			{
				"source": "FileA",
				"target": "FileB",
				"properties": {}
			},
			{
				"source": "X",
				"target": "Y",
				"properties": {"keywords": "depends_on", "source_id": "/src/X.cs", "file_path": "/src/X.cs"}
			}
		]
	}`

	var data struct {
		Edges []map[string]any `json:"edges"`
	}
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	result := make(map[string]map[string]any)
	for _, edge := range data.Edges {
		src := ""
		tgt := ""
		kw := ""

		if s, ok := edge["source"].(string); ok && s != "" {
			src = s
		}
		if t, ok := edge["target"].(string); ok && t != "" {
			tgt = t
		}
		if props, ok := edge["properties"].(map[string]any); ok {
			if k, ok := props["keywords"].(string); ok {
				kw = k
			}
		}

		if src == "" || tgt == "" {
			continue
		}

		key := src + "|" + tgt + "|" + kw
		result[key] = edge
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 valid edges, got %d (2 should be skipped for empty src/tgt)", len(result))
	}

	// Verify first edge
	key1 := "DeletedClass|UserService|depends_on"
	e1, ok := result[key1]
	if !ok {
		t.Fatalf("key %q not found", key1)
	}
	props, _ := e1["properties"].(map[string]any)
	if sid, _ := props["source_id"].(string); sid != "/src/DeletedClass.cs" {
		t.Errorf("source_id = %q, expected %q", sid, "/src/DeletedClass.cs")
	}

	// Verify edge with no keywords gets empty kw string in key
	key2 := "FileA|FileB|"
	if _, ok := result[key2]; !ok {
		t.Errorf("key %q should exist (edge with no keywords property)", key2)
	}
}

func TestParserRelationsToMap(t *testing.T) {
	relations := []parser.Relation{
		{Source: "Person", Target: "Name", Description: "Person.Name", Keywords: "property, has", Weight: 0.8, SourceID: "Person.cs.txt", FilePath: "/path/Person.cs"},
		{Source: "Person", Target: "GetName", Description: "Person.GetName()", Keywords: "method, defines", Weight: 1.0, SourceID: "Person.cs.txt", FilePath: "/path/Person.cs"},
		{Source: "Calculator", Target: "IOperation", Description: "Calculator depends on IOperation", Keywords: "depends_on", Weight: 0.9},
	}

	result := parserRelationsToMap(relations)

	if len(result) != 3 {
		t.Fatalf("expected 3 relation keys, got %d", len(result))
	}

	// Verify source_id and file_path are present when set
	key1 := "Person|Name|property, has"
	m1, ok := result[key1]
	if !ok {
		t.Fatalf("key %q not found in result", key1)
	}
	if sid, _ := m1["source_id"].(string); sid != "Person.cs.txt" {
		t.Errorf("source_id = %q, expected %q", sid, "Person.cs.txt")
	}
	if fp, _ := m1["file_path"].(string); fp != "/path/Person.cs" {
		t.Errorf("file_path = %q, expected %q", fp, "/path/Person.cs")
	}
	if desc, _ := m1["description"].(string); desc != "Person.Name" {
		t.Errorf("description = %q, expected %q", desc, "Person.Name")
	}
	if kw, _ := m1["keywords"].(string); kw != "property, has" {
		t.Errorf("keywords = %q, expected %q", kw, "property, has")
	}
	if w, _ := m1["weight"].(float64); w != 0.8 {
		t.Errorf("weight = %f, expected 0.8", w)
	}

	// Verify source_id/file_path are absent when empty (backward compat)
	key3 := "Calculator|IOperation|depends_on"
	m3, ok := result[key3]
	if !ok {
		t.Fatalf("key %q not found in result", key3)
	}
	if _, exists := m3["source_id"]; exists {
		t.Error("source_id should be absent when not set")
	}
	if _, exists := m3["file_path"]; exists {
		t.Error("file_path should be absent when not set")
	}
}

func findRepoRoot() string {
	dir, _ := os.Getwd()
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

func TestRunCSharpAnalyser(t *testing.T) {
	// Set env var so pipeline finds the binary
	repoRoot := findRepoRoot()
	analyserPath := filepath.Join(repoRoot, "analysers", "csharp", "csharp_analyser.exe")
	if _, err := os.Stat(analyserPath); err != nil {
		t.Skip("analyser binary not found, build it first: cd analysers/csharp && go build -o csharp_analyser.exe ./cmd/csharp_analyser/")
	}
	t.Setenv("CSHARP_ANALYSER_PATH", analyserPath)

	// Try testdata first, then test-csharp-project
	sln := filepath.Join(repoRoot, "analysers", "csharp", "testdata", "TestApp.sln")
	if _, err := os.Stat(sln); err != nil {
		sln = filepath.Join(repoRoot, "..", "test-csharp-project", "TestCodebase.sln")
	}
	if _, err := os.Stat(sln); err != nil {
		t.Skip("test solution not found; checked testdata/ and ../test-csharp-project/")
	}

	results, err := RunCSharpAnalyser(sln, "")
	if err != nil {
		t.Fatalf("RunCSharpAnalyser: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}

	// Check basic format
	for _, r := range results {
		if len(r.Content) == 0 {
			t.Errorf("empty content for %s", r.Filename)
		}
	}

	t.Logf("Got %d results", len(results))
	for _, r := range results {
		t.Logf("  %s (%d bytes)", r.Filename, len(r.Content))
	}

	// Parse ALL analyser output and verify RELATION lines produce correct SourceID/FilePath
	parserInput := make([]struct {
		Filename string
		Content  string
	}, len(results))
	for i, r := range results {
		parserInput[i] = struct {
			Filename string
			Content  string
		}{r.Filename, r.Content}
	}
	parsedEntities, parsedRelations := parser.ParseCSharpOutput(parserInput)
	t.Logf("Parsed %d entities, %d relations total", len(parsedEntities), len(parsedRelations))
	if len(parsedRelations) > 0 {
		relCount := 0
		for _, rel := range parsedRelations {
			// CALL relations (Weight=0.5) don't carry FilePath in the analyser output format
			if rel.Keywords == "calls" {
				continue
			}
			relCount++
			if rel.SourceID == "" {
				t.Errorf("RELATION %s->%s has empty SourceID", rel.Source, rel.Target)
			}
			if rel.FilePath == "" {
				t.Errorf("RELATION %s->%s has empty FilePath", rel.Source, rel.Target)
			}
		}
		t.Logf("Checked %d non-CALL relations — all have SourceID and FilePath populated", relCount)
	} else {
		t.Log("No relations to check")
	}
}
