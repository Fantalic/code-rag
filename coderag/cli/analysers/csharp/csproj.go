package csharpanalyser

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

type Project struct {
	XMLName       xml.Name        `xml:"Project"`
	ItemGroups    []ItemGroup     `xml:"ItemGroup"`
	PropertyGroup []PropertyGroup `xml:"PropertyGroup"`
}

type ItemGroup struct {
	Compiles    []Compile    `xml:"Compile"`
	References  []Reference  `xml:"Reference"`
	ProjectRefs []ProjectRef `xml:"ProjectReference"`
}

type Compile struct {
	Include string `xml:"Include,attr"`
}

type Reference struct {
	Include string `xml:"Include,attr"`
}

type ProjectRef struct {
	Include string `xml:"Include,attr"`
}

type PropertyGroup struct {
	RootNamespace string `xml:"RootNamespace"`
}

type ProjectFiles struct {
	SourceFiles []string
	ProjectDir  string
	ProjectRefs []string
}

func ParseProject(csprojPath string) (*ProjectFiles, error) {
	data, err := os.ReadFile(csprojPath)
	if err != nil {
		return nil, err
	}

	var proj Project
	if err := xml.Unmarshal(data, &proj); err != nil {
		return nil, err
	}

	projectDir := filepath.Dir(csprojPath)
	result := &ProjectFiles{
		ProjectDir: projectDir,
	}

	for _, ig := range proj.ItemGroups {
		for _, c := range ig.Compiles {
			path := c.Include
			if !filepath.IsAbs(path) {
				path = filepath.Join(projectDir, path)
			}
			result.SourceFiles = append(result.SourceFiles, filepath.Clean(path))
		}
		for _, pr := range ig.ProjectRefs {
			path := pr.Include
			if !filepath.IsAbs(path) {
				path = filepath.Join(projectDir, path)
			}
			result.ProjectRefs = append(result.ProjectRefs, filepath.Clean(path))
		}
	}

	// Filter to only .cs files
	var csFiles []string
	for _, f := range result.SourceFiles {
		if strings.EqualFold(filepath.Ext(f), ".cs") {
			csFiles = append(csFiles, f)
		}
	}
	result.SourceFiles = csFiles

	// SDK-style projects: no explicit <Compile> elements means all .cs files
	// under the project directory are implicitly included. Fall back to a
	// directory walk when no source files were found via XML.
	if len(result.SourceFiles) == 0 {
		csFiles = nil
		filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := info.Name()
				if name == "bin" || name == "obj" || name == "node_modules" || name == ".git" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".cs") {
				csFiles = append(csFiles, path)
			}
			return nil
		})
		result.SourceFiles = csFiles
	}

	return result, nil
}

// ProjectCoverage is the diagnostic record emitted per project. It captures
// what we did (and didn't) process so callers can surface coverage gaps.
type ProjectCoverage struct {
	ProjectPath string   // absolute path to .csproj (relative to solution dir in output)
	SourceFiles []string // files collected (absolute paths)
	ProjectRefs []string // <=Project> refs collected
	Warnings    []string // non-fatal problems (bad reads, missing globs, unparseable csproj, ...)
}

// CoverageReport aggregates diagnostics across the whole solution.
type CoverageReport struct {
	Projects []ProjectCoverage
}

// CoverageKeys surfaces the canonical human-friendly id used in COVERAGE|
// records: project file basename without extension.
func (c ProjectCoverage) CoverageKey(slnDir string) string {
	if rel, err := filepath.Rel(slnDir, c.ProjectPath); err == nil {
		rel = strings.TrimSuffix(rel, ".csproj")
		return filepath.ToSlash(filepath.Join("[project]", rel))
	}
	base := filepath.Base(c.ProjectPath)
	return strings.TrimSuffix(base, ".csproj")
}

func CollectSourceFiles(slnPath string) ([]string, error) {
	files, _, err := CollectSourceFilesWithCoverage(slnPath)
	return files, err
}

// CollectSourceFilesWithCoverage returns the same file list as
// CollectSourceFiles plus a per-project diagnostic record. Both outputs share
// the same iteration order so coverage rows can be matched 1:1 with files.
func CollectSourceFilesWithCoverage(slnPath string) ([]string, CoverageReport, error) {
	sln, err := ParseSolution(slnPath)
	if err != nil {
		return nil, CoverageReport{}, err
	}

	var allFiles []string
	seen := make(map[string]bool)
	var report CoverageReport

	for _, projPath := range sln.ProjectPaths {
		pf, err := ParseProject(projPath)
		if err != nil {
			report.Projects = append(report.Projects, ProjectCoverage{
				ProjectPath: projPath,
				Warnings:    []string{"parse_error: " + err.Error()},
			})
			continue
		}
		cov := ProjectCoverage{
			ProjectPath: pf.ProjectDir,
			ProjectRefs: pf.ProjectRefs,
		}
		for _, f := range pf.SourceFiles {
			abs, _ := filepath.Abs(f)
			if !seen[abs] {
				seen[abs] = true
				allFiles = append(allFiles, abs)
			}
			cov.SourceFiles = append(cov.SourceFiles, abs)
		}
		if len(pf.SourceFiles) == 0 {
			cov.Warnings = append(cov.Warnings, "no_source_files_found")
		}
		report.Projects = append(report.Projects, cov)
	}

	return allFiles, report, nil
}
