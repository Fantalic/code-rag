package main

import (
	"fmt"
	"os"

	"github.com/code-lightrag/coderag/pkg/watchdog/pipeline"
	"github.com/code-lightrag/coderag/pkg/watchdog/parser"
)

func main() {
	sln := os.Args[1]
	results, err := pipeline.RunCSharpAnalyser(sln)
	if err != nil {
		fmt.Fprintf(os.Stderr, "RunCSharpAnalyser error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Results: %d files\n", len(results))
	for _, r := range results {
		fmt.Printf("  %s (%d bytes)\n", r.Filename, len(r.Content))
	}

	parserResults := make([]struct{ Filename string; Content string }, len(results))
	for i, r := range results {
		parserResults[i] = struct{ Filename string; Content string }{r.Filename, r.Content}
	}

	entities, relations := parser.ParseCSharpOutput(parserResults)
	fmt.Printf("\nParsed: %d entities, %d relations\n", len(entities), len(relations))
	for _, e := range entities {
		fmt.Printf("  ENTITY: %s (%s) file=%s\n", e.Name, e.EntityType, e.FilePath)
	}
	for _, r := range relations {
		fmt.Printf("  REL: %s -> %s [%s]\n", r.Source, r.Target, r.Keywords)
	}
}
