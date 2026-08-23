package csharpanalyser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const headerPrefix = "# EXPORTS:"

// OutputFormatVersion marks entity lines carrying an end-line column.
// Format 1: TYPE|Name|FilePath|LineNo|Description
// Format 2: TYPE|Name|FilePath|StartLine|EndLine|Description
// The ingestion-side parser (cli/parser) reads the marker and stays
// tolerant of old analyser outputs during upgrades.
const OutputFormatVersion = 2

func fileMTIME(filePath string) int64 {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}

type OutputConfig struct {
	OutputDir string
	SlnDir    string
}

func NewOutputConfig(dir string, slnDir string) *OutputConfig {
	return &OutputConfig{OutputDir: dir, SlnDir: slnDir}
}

func (oc *OutputConfig) WriteFileResult(fr *FileResult) (string, error) {
	relPath, err := filepath.Rel(oc.SlnDir, fr.FilePath)
	if err != nil {
		relPath = filepath.Base(fr.FilePath)
	}
	flatName := strings.ReplaceAll(relPath, "\\", "__")
	flatName = strings.ReplaceAll(flatName, "/", "__")
	outPath := filepath.Join(oc.OutputDir, flatName+".txt")
	if err := os.MkdirAll(oc.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

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
		fmt.Fprintf(out, "CALL|%s|%s|%s|%s|%d\n", c.CallerFile, c.CallerMethod, c.CalleeName, c.CalleeClass, c.LineNo)
	}

	return outPath, nil
}

// WriteCoverageReport writes a per-project diagnostics sidecar in COVERAGE|
// rows so the downstream parser can ingest it into COVERAGE entities.
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
	rels := make([]string, 0, len(report.Projects))
	for i := range report.Projects {
		rels = append(rels, report.Projects[i].CoverageKey(oc.SlnDir))
	}
	_ = rels
	fmt.Fprintf(out, "# Total Projects: %d\n", len(report.Projects))

	// Stable project-keyed rows: COVERAGE|<key>|<json-ish-fields>
	// The description column holds a compact, machine-friendly summary that
	// the parser keeps verbatim in the COVERAGE entity.
	for _, p := range report.Projects {
		key := p.CoverageKey(oc.SlnDir)
		warnings := strings.Join(p.Warnings, ";")
		if warnings == "" {
			warnings = "-"
		}
		// Description column is JSON so the indexer can treat each project
		// as a structured COVERAGE entity without per-tool parsers.
		fmt.Fprintf(out, "COVERAGE|%s|{\"files\":%d,\"refs\":%d,\"warnings\":\"%s\"}\n",
			key, len(p.SourceFiles), len(p.ProjectRefs), warnings)
	}
	return outPath, nil
}

func (oc *OutputConfig) WriteFullIndex(index *GlobalIndex, files []string) (string, error) {
	outPath := filepath.Join(oc.OutputDir, "_index.txt")
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
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

	fmt.Fprintln(out, "\n# CLASSES")
	for name, ci := range index.Classes {
		baseInfo := ""
		if ci.BaseType != "" {
			baseInfo = " : " + ci.BaseType
		}
		if len(ci.Interfaces) > 0 {
			baseInfo += " : " + strings.Join(ci.Interfaces, ", ")
		}
		fmt.Fprintf(out, "%s|%s|%d%s\n", name, ci.FilePath, ci.LineNo, baseInfo)
	}

	fmt.Fprintln(out, "\n# METHODS")
	for name, methods := range index.Methods {
		for _, m := range methods {
			fmt.Fprintf(out, "%s|%s|%d|%s\n", name, m.FilePath, m.LineNo, m.ReturnType)
		}
	}

	fmt.Fprintln(out, "\n# PROPERTIES")
	for name, props := range index.Properties {
		for _, p := range props {
			fmt.Fprintf(out, "%s|%s|%d|%s\n", name, p.FilePath, p.LineNo, p.Type)
		}
	}

	return outPath, nil
}

func ComputeDiff(oldLines, newLines []string) *DiffReport {
	oldSet := make(map[string]bool)
	for _, l := range oldLines {
		oldSet[l] = true
	}
	newSet := make(map[string]bool)
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

	return report
}

// WriteChangeLog appends one CHANGELOG|...|{json} row per non-empty diff
// action. The downstream parser turns these into CHANGELOG entities that the
// MCP what_changed tool filters by timestamp.
func (oc *OutputConfig) WriteChangeLog(filePath string, diff *DiffReport, ts int64) (string, error) {
	if oc == nil {
		return "", fmt.Errorf("nil output config")
	}
	if diff == nil || diff.IsEmpty() {
		return "", nil // nothing to do; no sidecar touched
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

	rows := ChangelogRows(filePath, diff, ts)
	for _, row := range rows {
		if _, err := fmt.Fprintln(out, row); err != nil {
			return outPath, err
		}
	}
	return outPath, nil
}

// ChangelogRows serialises a diff into CHANGELOG-stable rows. Exposed as a
// pure function so tests can assert what gets written without touching disk.
//
// Format: CHANGELOG|<relative-file>|<json>
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
	// Description column is JSON so the MCP server can filter by changed_at
	// without parse-by-prefix heuristics per series.
	desc := fmt.Sprintf(`{"file":%q,"action":%q,"symbol":%q,"changed_at":%d}`,
		file, action, symbol, ts)
	return fmt.Sprintf("CHANGELOG|%s|%s", filepath.ToSlash(filepath.Clean(file)), desc)
}

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
		line := scanner.Text()
		if strings.HasPrefix(line, headerPrefix) {
			exportsPart := strings.TrimSpace(strings.TrimPrefix(line, headerPrefix))
			if exportsPart == "" {
				return nil, nil
			}
			exports := strings.Split(exportsPart, ",")
			var result []string
			for _, e := range exports {
				e = strings.TrimSpace(e)
				if e != "" {
					result = append(result, e)
				}
			}
			return result, nil
		}
	}
	return nil, nil
}
