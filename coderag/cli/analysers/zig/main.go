package ziganalyser

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CLIConfig holds the parsed command-line flags for a single invocation.
type CLIConfig struct {
	Command   string
	Dir       string
	Changed   string
	OutputDir string
}

// ParseCLI reads the argv slice and returns a populated config.
// Expected shape: zig_analyser <scan|update> [flags]
func ParseCLI(args []string) (*CLIConfig, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("usage: zig_analyser <scan|update> [flags]")
	}

	cfg := &CLIConfig{
		Command: args[1],
	}

	fs := flag.NewFlagSet("zig_analyser", flag.ContinueOnError)
	fs.StringVar(&cfg.Dir, "dir", ".", "Project directory containing .zig files")
	fs.StringVar(&cfg.Changed, "changed", "", "Comma-separated changed files (for update)")
	fs.StringVar(&cfg.OutputDir, "output", "", "Output directory (default: <project>/.lightrag)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: zig_analyser <scan|update> [flags]\n")
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  scan    Full scan of directory, write output files\n")
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

// Run dispatches the chosen command. Zig projects have no .sln/.csproj
// equivalent so the project root is just a directory containing .zig
// files; the analyser walks it recursively (skipping cache/zig-out).
func Run(cfg *CLIConfig) error {
	absDir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return fmt.Errorf("resolve project dir: %w", err)
	}

	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(absDir, ".lightrag")
	}

	outputCfg := NewOutputConfig(outputDir, absDir)

	switch cfg.Command {
	case "scan":
		return runScan(absDir, outputCfg)
	case "update":
		return runUpdate(absDir, outputCfg, cfg.Changed)
	default:
		return fmt.Errorf("unknown command: %s (use scan or update)", cfg.Command)
	}
}

func runScan(projectDir string, outputCfg *OutputConfig) error {
	files, coverage, err := CollectZigFilesWithCoverage(projectDir)
	if err != nil {
		return fmt.Errorf("collect zig files: %w", err)
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "warning: no .zig files found in %s\n", projectDir)
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

	// Coverage sidecar — always written so the MCP `index_coverage` tool
	// sees a single project even when zero .zig files were collected.
	if covPath, err := outputCfg.WriteCoverageReport(coverage); err != nil {
		fmt.Fprintf(os.Stderr, "warning: write coverage: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "wrote %s\n", covPath)
	}

	fmt.Printf("OK: scanned %d files\n", len(files))
	return nil
}

func runUpdate(projectDir string, outputCfg *OutputConfig, changed string) error {
	var changedFiles []string
	if changed != "" {
		for _, p := range strings.Split(changed, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if abs, err := filepath.Abs(p); err == nil {
				p = abs
			}
			changedFiles = append(changedFiles, p)
		}
	}

	if len(changedFiles) == 0 {
		return fmt.Errorf("no changed files specified (use -changed flag)")
	}

	allFiles, err := CollectZigFiles(projectDir)
	if err != nil {
		return fmt.Errorf("collect zig files: %w", err)
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

		oldOutputPath := outputCfg.OutputPathFor(cf)
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
			rel, _ := filepath.Rel(projectDir, cf)
			if _, err := outputCfg.WriteChangeLog(rel, diff, nowUnix()); err != nil {
				fmt.Fprintf(os.Stderr, "warning: write changelog for %s: %v\n", cf, err)
			}
		}
	}

	return nil
}

// Main is the package-level entry: parse argv, run the chosen command,
// return an exit code. Mirrors the C# analyser's Main() so the binary
// in cmd/zig_analyser is a one-liner.
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
