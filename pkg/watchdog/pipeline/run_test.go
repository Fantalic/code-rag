package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

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

	sln := filepath.Join(repoRoot, "analysers", "csharp", "testdata", "TestApp.sln")
	if _, err := os.Stat(sln); err != nil {
		t.Skip("test solution not found")
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
}
