package ziganalyser

// Output writer + diff computation. Writes the same `# FORMAT: 2`
// layout as the C# analyser so cli/parser can ingest both with the
// same code path. Per-file output is the single source of truth; the
// pipeline reads only the EXPORTS header to schedule re-analysis.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const headerPrefix = "# EXPORTS:"

// OutputFormatVersion is the entity-line format. Matches the C#
// analyser so the parser can read both with one schema.
const OutputFormatVersion = 2

// OutputConfig points the writer at a directory + a project root used
// to compute relative paths for the coverage/index sidecars.
type OutputConfig struct {
	OutputDir string
	ProjectDir string
}

func NewOutputConfig(dir, projectDir string) *OutputConfig {
	return &OutputConfig{OutputDir: dir, ProjectDir: projectDir}
}

// OutputPathFor returns the flat-named .txt path for a given input
// file, e.g. "src/main.zig" -> "<dir>/src__main.zig.txt".
func (oc *OutputConfig) OutputPathFor(filePath string) string {
	relPath, err := filepath.Rel(oc.ProjectDir, filePath)
	if err != nil {
		relPath = filepath.Base(filePath)
	}
	flatName := strings.ReplaceAll(relPath, "\\", "__")
	flatName = strings.ReplaceAll(flatName, "/", "__")
	return filepath.Join(oc.OutputDir, flatName+".txt")
}

// WriteFileResult serialises one FileResult to a .txt sidecar.
func (oc *OutputConfig) WriteFileResult(fr *FileResult) (string, error) {
	if err := os.MkdirAll(oc.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	outPath := oc.OutputPathFor(fr.FilePath)
	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create output file %s: %w", outPath, err)
	}
	defer out.Close()

	sort.Strings(fr.Exports)
	sort.Strings(fr.Imports)

	fmt.Fprintf(out, "# FORMAT: %d\n", OutputFormatVersion)
	fmt.Fprintf(out, "# EXPORTS: %s\n", strings.Join(fr.Exports, ", "))
	if mtime := fileMTIME(fr.FilePath); mtime > 0 {
		fmt.Fprintf(out, "# MTIME: %d\n", mtime)
	}
	fmt.Fprintf(out, "# IMPORTS: %s\n", strings.Join(fr.Imports, ", "))

	for _, e := range fr.Entities {
		endLine := e.EndLine
		if endLine < e.LineNo {
			endLine = e.LineNo
		}
		fmt.Fprintf(out, "%s|%s|%s|%d|%d|%s\n", e.Type, e.Name, e.FilePath, e.LineNo, endLine, e.Description)
	}

	for _, r := range fr.Relations {
		fmt.Fprintf(out, "RELATION|%s|%s|%s|%s|%s|%.1f\n", r.Source, r.Target, r.Description, r.Keywords, fr.FilePath, r.Weight)
	}

	for _, c := range fr.Calls {
		fmt.Fprintf(out, "CALL|%s|%s|%s|%s|%d\n", c.CallerFile, c.CallerMethod, c.CalleeName, c.CalleeOwner, c.LineNo)
	}

	return outPath, nil
}

// WriteCoverageReport writes a per-project diagnostics sidecar in
// COVERAGE| rows so the downstream parser ingests it into COVERAGE
// entities. Mirrors the C# analyser sidecar schema exactly.
func (oc *OutputConfig) WriteCoverageReport(report CoverageReport) (string, error) {
	if oc == nil {
		return "", fmt.Errorf("nil output config")
	}
	if err := os.MkdirAll(oc.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	outPath := filepath.Join(oc.OutputDir, "_coverage.txt")
	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create coverage file: %w", err)
	}
	defer out.Close()

	fmt.Fprintln(out, "# COVERAGE REPORT")
	fmt.Fprintf(out, "# Total Projects: %d\n", len(report.Projects))

	for _, proj := range report.Projects {
		key := proj.CoverageKey(oc.ProjectDir)
		warnings := strings.Join(proj.Warnings, ";")
		if warnings == "" {
			warnings = "-"
		}
		// Description column is JSON-shaped so the indexer can treat
		// each project as a structured COVERAGE entity without per-tool
		// parsers. `skipped` is a per-file list (path: reason) and
		// `scanned_at` is the unix timestamp of the scan.
		skippedJSON, _ := json.Marshal(proj.Skipped)
		if proj.Skipped == nil {
			skippedJSON = []byte("[]")
		}
		fmt.Fprintf(out, "COVERAGE|%s|{\"files\":%d,\"skipped\":%s,\"scanned_at\":%d,\"warnings\":\"%s\"}\n",
			key, len(proj.SourceFiles), string(skippedJSON), proj.ScannedAt, warnings)
	}
	return outPath, nil
}

// WriteFullIndex writes the global symbol index for debugging +
// manual inspection. Not used by the pipeline but useful when
// investigating "why didn't this resolve?" questions.
func (oc *OutputConfig) WriteFullIndex(index *GlobalIndex, files []string) (string, error) {
	outPath := filepath.Join(oc.OutputDir, "_index.txt")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create index file: %w", err)
	}
	defer out.Close()

	fmt.Fprintln(out, "# GLOBAL INDEX")
	fmt.Fprintln(out, "# File: _index.txt")
	fmt.Fprintln(out, "# Total files:", len(files))

	fmt.Fprintln(out, "\n# FUNCTIONS")
	for name, fns := range index.Functions {
		for _, f := range fns {
			fmt.Fprintf(out, "%s|%s|%d|%s\n", name, f.FilePath, f.LineNo, f.Owner)
		}
	}

	fmt.Fprintln(out, "\n# FIELDS")
	for name, fields := range index.Fields {
		for _, f := range fields {
			fmt.Fprintf(out, "%s|%s|%d|%s\n", name, f.FilePath, f.LineNo, f.Owner)
		}
	}

	fmt.Fprintln(out, "\n# TYPES")
	for name, t := range index.Types {
		fmt.Fprintf(out, "%s|%s|%d|%s\n", name, t.FilePath, t.LineNo, t.Kind)
	}

	return outPath, nil
}

// ComputeDiff compares old vs new exports and returns the symbols that
// disappeared (RemovedSymbols) or appeared (AddedSymbols).
func ComputeDiff(oldLines, newLines []string) *DiffReport {
	oldSet := make(map[string]bool, len(oldLines))
	for _, l := range oldLines {
		oldSet[l] = true
	}
	newSet := make(map[string]bool, len(newLines))
	for _, l := range newLines {
		newSet[l] = true
	}

	report := &DiffReport{}
	for k := range oldSet {
		if !newSet[k] {
			report.RemovedSymbols = append(report.RemovedSymbols, k)
		}
	}
	for k := range newSet {
		if !oldSet[k] {
			report.AddedSymbols = append(report.AddedSymbols, k)
		}
	}
	sort.Strings(report.AddedSymbols)
	sort.Strings(report.RemovedSymbols)
	return report
}

// WriteChangeLog appends one CHANGELOG|<file>|<json> row per
// added/removed symbol. The downstream parser ingests these into
// CHANGELOG entities the MCP what_changed tool can filter by
// timestamp.
func (oc *OutputConfig) WriteChangeLog(filePath string, diff *DiffReport, ts int64) (string, error) {
	if oc == nil {
		return "", fmt.Errorf("nil output config")
	}
	if diff == nil || diff.IsEmpty() {
		return "", nil
	}
	if err := os.MkdirAll(oc.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	outPath := filepath.Join(oc.OutputDir, "_changelog.txt")
	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("open changelog: %w", err)
	}
	defer out.Close()

	for _, row := range ChangelogRows(filePath, diff, ts) {
		if _, err := fmt.Fprintln(out, row); err != nil {
			return outPath, err
		}
	}
	return outPath, nil
}

// ChangelogRows serialises a diff into CHANGELOG-stable rows. Format:
// CHANGELOG|<relative-file>|<json>.
func ChangelogRows(filePath string, diff *DiffReport, ts int64) []string {
	rel := filePath
	if rel == "" {
		rel = "<unknown>"
	}
	var rows []string
	for _, sym := range diff.AddedSymbols {
		rows = append(rows, changelogRow(rel, "added", sym, ts))
	}
	for _, sym := range diff.RemovedSymbols {
		rows = append(rows, changelogRow(rel, "removed", sym, ts))
	}
	return rows
}

func changelogRow(file, action, symbol string, ts int64) string {
	desc := fmt.Sprintf(`{"file":%q,"action":%q,"symbol":%q,"changed_at":%d}`,
		file, action, symbol, ts)
	return fmt.Sprintf("CHANGELOG|%s|%s", filepath.ToSlash(filepath.Clean(file)), desc)
}

// ReadOutputExports reads the first EXPORTS header from a previously
// written .txt sidecar. Returns nil with no error when the file is
// missing (treat the file as "no previous state").
func ReadOutputExports(outputPath string) ([]string, error) {
	f, err := os.Open(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		// First line is "# FORMAT: 2" — skip it explicitly so a stale
		// header doesn't accidentally satisfy the EXPORTS check.
	}
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, headerPrefix) {
			body := strings.TrimSpace(strings.TrimPrefix(line, headerPrefix))
			if body == "" {
				return nil, nil
			}
			parts := strings.Split(body, ",")
			var out []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					out = append(out, p)
				}
			}
			return out, nil
		}
	}
	return nil, nil
}

func fileMTIME(filePath string) int64 {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}
