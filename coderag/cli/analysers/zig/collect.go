package ziganalyser

// Filesystem discovery. Walks the project directory looking for
// `.zig` source files, skipping build artefacts. Mirrors the C#
// analyser's CollectSourceFiles with the Zig-specific cache list.

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// CoverageReport aggregates per-project diagnostics for the
// `_coverage.txt` sidecar. Mirrors the C# CoverageReport shape so the
// downstream parser can ingest both identically.
type CoverageReport struct {
	Projects []ProjectCoverage
}

// ProjectCoverage records the outcome of one project's discovery.
type ProjectCoverage struct {
	ProjectDir  string
	SourceFiles []string
	Warnings    []string
	// Skipped holds per-file skip entries of the form "path: reason"
	// (parse failures etc.). Empty when every discovered file parsed.
	Skipped []string
	// ScannedAt is the unix timestamp of the scan that produced this
	// report, so `index_coverage` can surface freshness.
	ScannedAt int64
}

// CoverageKey is the stable identifier written into COVERAGE rows.
func (p *ProjectCoverage) CoverageKey(root string) string {
	if rel, err := filepath.Rel(root, p.ProjectDir); err == nil && rel != "" {
		return filepath.ToSlash(rel)
	}
	return filepath.Base(p.ProjectDir)
}

// CollectZigFilesWithCoverage walks the project directory and returns
// every .zig file under it (excluding zig-out / .zig-cache) plus a
// coverage report describing the discovery outcome. A single project
// entry is always returned, even when the directory has zero files,
// so the MCP `index_coverage` tool can surface "no source found".
func CollectZigFilesWithCoverage(projectDir string) ([]string, CoverageReport, error) {
	report := CoverageReport{Projects: []ProjectCoverage{
		{ProjectDir: projectDir},
	}}
	var files []string

	err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			// Skip Zig's standard build cache, the test cache used by
			// `zig build test` (named .zig-cache_test to keep it
			// separate from the build cache), the project-local
			// zig-out directory, and the global package cache
			// (`zig-pkg`) used by `zig fetch`.
			if name == "zig-out" || name == ".zig-cache" || name == ".zig-cache_test" || name == "zig-pkg" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".zig") {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		files = append(files, abs)
		return nil
	})
	if err != nil {
		return nil, report, fmt.Errorf("walk project: %w", err)
	}

	report.Projects[0].SourceFiles = files
	if len(files) == 0 {
		report.Projects[0].Warnings = append(report.Projects[0].Warnings, "no .zig files found")
	}
	return files, report, nil
}
