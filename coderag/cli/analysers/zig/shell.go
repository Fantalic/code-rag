package ziganalyser

// End-to-end shell helper. Exposed so test code (in the parent
// coderag/cli/parser package) can drive the analyser binary directly
// without re-implementing flag parsing.

import (
	"fmt"
	"os/exec"
)

// ShellOut runs the zig_analyser binary at `binaryPath` with `scan`
// against `projectDir`, writing results to `outputDir`. Returns the
// captured stdout (analyser prints progress and diff lines) and any
// exec error.
func ShellOut(binaryPath, projectDir, outputDir string) (string, error) {
	cmd := exec.Command(binaryPath,
		"scan",
		"--dir", projectDir,
		"--output", outputDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("zig_analyser scan: %w", err)
	}
	return string(out), nil
}
