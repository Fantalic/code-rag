package main

// dumper binary entry point.

import (
	"fmt"
	"os"

	ziganalyser "github.com/coderag/analysers/zig"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dumper <file.zig>")
		os.Exit(1)
	}
	fr, err := ziganalyser.DumpFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("File: %s\n", fr.FilePath)
	fmt.Printf("Entities (%d):\n", len(fr.Entities))
	for _, e := range fr.Entities {
		fmt.Printf("  %s | %s | %s | %d-%d | %s\n", e.Type, e.Name, e.FilePath, e.LineNo, e.EndLine, e.Description)
	}
	fmt.Printf("Relations (%d):\n", len(fr.Relations))
	for _, r := range fr.Relations {
		fmt.Printf("  %s -> %s (%s)\n", r.Source, r.Target, r.Keywords)
	}
	fmt.Printf("Exports: %v\n", fr.Exports)
	fmt.Printf("Imports: %v\n", fr.Imports)
}
