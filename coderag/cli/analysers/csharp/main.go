package csharpanalyser

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CLIConfig struct {
	Command   string
	Solution  string
	Dir       string
	Changed   string
	OutputDir string
}

func ParseCLI(args []string) (*CLIConfig, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("usage: csharp_analyser <scan|update> [flags]")
	}

	cfg := &CLIConfig{
		Command: args[1],
	}

	fs := flag.NewFlagSet("csharp_analyser", flag.ContinueOnError)
	fs.StringVar(&cfg.Solution, "solution", "", "Path to solution file")
	fs.StringVar(&cfg.Dir, "dir", ".", "Project directory")
	fs.StringVar(&cfg.Changed, "changed", "", "Comma-separated changed files (for update)")
	fs.StringVar(&cfg.OutputDir, "output", "", "Output directory (default: <project>/.lightrag)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: csharp_analyser <scan|update> [flags]\n")
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  scan    Full scan of solution, write output files\n")
		fmt.Fprintf(os.Stderr, "  update  Incremental update for changed files, emit diff to stdout\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		fs.PrintDefaults()
	}

	if len(args) <= 2 {
		return cfg, nil
	}

	if err := fs.Parse(args[2:]); err != nil {
		return nil, err
	}

	return cfg, nil
}

func Run(cfg *CLIConfig) error {
	slnPath := cfg.Solution
	if slnPath == "" {
		slnPath = FindSlnFile(cfg.Dir)
		if slnPath == "" {
			return fmt.Errorf("no solution file found in %s or parent dirs", cfg.Dir)
		}
	}

	absSln, err := filepath.Abs(slnPath)
	if err != nil {
		return fmt.Errorf("resolve solution path: %w", err)
	}

	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(filepath.Dir(absSln), ".lightrag")
	}

	slnDir := filepath.Dir(absSln)
	outputCfg := NewOutputConfig(outputDir, slnDir)

	switch cfg.Command {
	case "scan":
		return runScan(absSln, outputCfg)
	case "update":
		return runUpdate(absSln, outputCfg, cfg.Changed)
	default:
		return fmt.Errorf("unknown command: %s (use scan or update)", cfg.Command)
	}
}

func runScan(slnPath string, outputCfg *OutputConfig) error {
	files, err := CollectSourceFiles(slnPath)
	if err != nil {
		return fmt.Errorf("collect source files: %w", err)
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "warning: no .cs files found in solution %s\n", slnPath)
		return nil
	}

	idxBuilder := NewIndexBuilder()
	index, err := idxBuilder.BuildIndex(files)
	if err != nil {
		return fmt.Errorf("build index: %w", err)
	}

	for _, fp := range files {
		fr, err := ParseFile(fp, index)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: parse %s: %v\n", fp, err)
			continue
		}
		path, err := outputCfg.WriteFileResult(fr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: write %s: %v\n", fp, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	}

	idxPath, err := outputCfg.WriteFullIndex(index, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: write index: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "wrote %s\n", idxPath)
	}

	fmt.Printf("OK: scanned %d files\n", len(files))
	return nil
}

func runUpdate(slnPath string, outputCfg *OutputConfig, changed string) error {
	var changedFiles []string
	if changed != "" {
		parts := strings.Split(changed, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				abs, err := filepath.Abs(p)
				if err == nil {
					p = abs
				}
				changedFiles = append(changedFiles, p)
			}
		}
	}

	if len(changedFiles) == 0 {
		return fmt.Errorf("no changed files specified (use -changed flag)")
	}

	needsRescan := make(map[string]bool)
	for _, cf := range changedFiles {
		needsRescan[cf] = true
	}

	allFiles, err := CollectSourceFiles(slnPath)
	if err != nil {
		return fmt.Errorf("collect source files: %w", err)
	}

	for _, af := range allFiles {
		abs, _ := filepath.Abs(af)
		if needsRescan[abs] || needsRescan[af] {
			needsRescan[abs] = true
		}
	}

	idxBuilder := NewIndexBuilder()
	index, err := idxBuilder.BuildIndex(allFiles)
	if err != nil {
		return fmt.Errorf("rebuild index: %w", err)
	}

	for _, cf := range changedFiles {
		fr, err := ParseFile(cf, index)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: parse %s: %v\n", cf, err)
			continue
		}

		relPath, _ := filepath.Rel(filepath.Dir(slnPath), cf)
		flatName := strings.ReplaceAll(relPath, "\\", "__")
		flatName = strings.ReplaceAll(flatName, "/", "__")
		oldOutputPath := filepath.Join(outputCfg.OutputDir, flatName+".txt")

		oldExports, _ := ReadOutputExports(oldOutputPath)

		path, err := outputCfg.WriteFileResult(fr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: write %s: %v\n", cf, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", path)

		diff := ComputeDiff(oldExports, fr.Exports)
		if !diff.IsEmpty() {
			fmt.Print(diff.String())
		}
	}

	return nil
}

func Main() int {
	cfg, err := ParseCLI(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if err := Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}
