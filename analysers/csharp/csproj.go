package csharpanalyser

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

type Project struct {
	XMLName       xml.Name         `xml:"Project"`
	ItemGroups    []ItemGroup      `xml:"ItemGroup"`
	PropertyGroup []PropertyGroup  `xml:"PropertyGroup"`
}

type ItemGroup struct {
	Compiles      []Compile       `xml:"Compile"`
	References    []Reference     `xml:"Reference"`
	ProjectRefs   []ProjectRef    `xml:"ProjectReference"`
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
	SourceFiles    []string
	ProjectDir     string
	ProjectRefs    []string
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

func CollectSourceFiles(slnPath string) ([]string, error) {
	sln, err := ParseSolution(slnPath)
	if err != nil {
		return nil, err
	}

	var allFiles []string
	seen := make(map[string]bool)

	for _, projPath := range sln.ProjectPaths {
		pf, err := ParseProject(projPath)
		if err != nil {
			continue
		}
		for _, f := range pf.SourceFiles {
			abs, _ := filepath.Abs(f)
			if !seen[abs] {
				seen[abs] = true
				allFiles = append(allFiles, abs)
			}
		}
	}

	return allFiles, nil
}
