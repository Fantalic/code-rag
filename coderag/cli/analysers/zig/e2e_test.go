package ziganalyser

// End-to-end smoke test that shells out to the freshly-built binary,
// then parses its output back through the same reader the parent
// pipeline uses. Skipped when the binary is missing.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndToEndScanAndParse(t *testing.T) {
	repoRoot := findRepoRoot(t)
	binary := filepath.Join(repoRoot, "zig_analyser.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("zig_analyser.exe not built; run `go build -o zig_analyser.exe ./cmd/zig_analyser/` first")
	}

	projectDir := filepath.Join(repoRoot, "testdata", "ZigApp")
	if _, err := os.Stat(projectDir); err != nil {
		t.Skipf("ZigApp fixture missing at %s: %v", projectDir, err)
	}

	out := t.TempDir()
	if _, err := ShellOut(binary, projectDir, out); err != nil {
		t.Fatalf("ShellOut: %v", err)
	}

	// Read every .txt sidecar (excluding _index/_coverage/_changelog).
	matches, err := filepath.Glob(filepath.Join(out, "*.txt"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var results []struct{ Filename, Content string }
	for _, m := range matches {
		name := filepath.Base(m)
		if strings.HasPrefix(name, "_") {
			continue
		}
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		results = append(results, struct{ Filename, Content string }{name, string(data)})
	}
	if len(results) == 0 {
		t.Fatal("analyser produced no .txt sidecars")
	}

	// Sanity: every ZigApp entity type must show up across the merged
	// set, and at least one `calls` relation must round-trip.
	seenTypes := map[string]bool{}
	callsFound := 0
	for _, r := range results {
		for _, line := range strings.Split(r.Content, "\n") {
			if !strings.HasPrefix(line, "FUNCTION|") &&
				!strings.HasPrefix(line, "STRUCT|") &&
				!strings.HasPrefix(line, "ENUM|") &&
				!strings.HasPrefix(line, "ERROR_SET|") &&
				!strings.HasPrefix(line, "CONST|") &&
				!strings.HasPrefix(line, "TEST|") &&
				!strings.HasPrefix(line, "IMPORT|") &&
				!strings.HasPrefix(line, "ENUM_MEMBER|") &&
				!strings.HasPrefix(line, "CALL|") {
				continue
			}
			head := strings.SplitN(line, "|", 2)[0]
			seenTypes[head] = true
			if head == "CALL" {
				callsFound++
			}
		}
	}

	for _, want := range []string{"FUNCTION", "STRUCT", "ENUM", "ERROR_SET", "CONST", "TEST", "IMPORT", "ENUM_MEMBER", "CALL"} {
		if !seenTypes[want] {
			t.Errorf("expected %q lines in output, missing", want)
		}
	}
	if callsFound == 0 {
		t.Error("expected at least one CALL row, got none")
	}
}

// findRepoRoot walks up from the test directory until a go.mod file
// for this nested module is found.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, _ := os.Getwd()
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
				if strings.Contains(string(data), "module github.com/coderag/analysers/zig") {
					return dir
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("could not locate zig analyser module from " + cwd)
	return ""
}
