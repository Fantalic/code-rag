package main

// zig_analyser binary entry point. Delegates everything to the package
// so tests can drive Main() without spawning a subprocess.

import (
	"os"

	ziganalyser "github.com/coderag/analysers/zig"
)

func main() {
	os.Exit(ziganalyser.Main())
}
