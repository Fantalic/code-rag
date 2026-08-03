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

	fmt.Fprintf(out, "# EXPORTS: %s\n", strings.Join(fr.Exports, ", "))
	fmt.Fprintf(out, "# IMPORTS: %s\n", strings.Join(fr.Imports, ", "))

	for _, e := range fr.Entities {
		fmt.Fprintf(out, "%s|%s|%s|%d|%s\n", e.Type, e.Name, e.FilePath, e.LineNo, e.Description)
	}

	for _, r := range fr.Relations {
		fmt.Fprintf(out, "RELATION|%s|%s|%s|%s|%s|%.1f\n", r.Source, r.Target, r.Description, r.Keywords, fr.FilePath, r.Weight)
	}

	for _, c := range fr.Calls {
		fmt.Fprintf(out, "CALL|%s|%s|%s|%s|%d\n", c.CallerFile, c.CallerMethod, c.CalleeName, c.CalleeClass, c.LineNo)
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
