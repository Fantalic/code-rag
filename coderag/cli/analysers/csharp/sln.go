package csharpanalyser

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var slnProjectRegex = regexp.MustCompile(`Project\("\{[A-F0-9-]+\}"\)\s*=\s*"([^"]+)",\s*"([^"]+\.csproj)"`)

type SolutionProjects struct {
	ProjectPaths []string
	SolutionDir  string
}

func ParseSolution(slnPath string) (*SolutionProjects, error) {
	f, err := os.Open(slnPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	solutionDir := filepath.Dir(slnPath)
	var paths []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		matches := slnProjectRegex.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		projPath := matches[2]
		if !filepath.IsAbs(projPath) {
			projPath = filepath.Join(solutionDir, projPath)
		}
		projPath = filepath.Clean(projPath)
		paths = append(paths, projPath)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &SolutionProjects{
		ProjectPaths: paths,
		SolutionDir:  solutionDir,
	}, nil
}

func FindSlnFile(dir string) string {
	dir = filepath.Clean(dir)
	for i := 0; i < 10; i++ {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ""
		}
		for _, e := range entries {
			if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".sln") {
				return filepath.Join(dir, e.Name())
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
