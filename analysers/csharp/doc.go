// Package csharpanalyser implements a Go-based C# static analyser that
// extracts entities (classes, methods, properties) and relationships
// (inheritance, implementation, calls) from C# codebases.
//
// It outputs structured text files (one per .cs file) that serve as the
// single source of truth for the codeRAG knowledge graph pipeline.
// The output includes # EXPORTS / # IMPORTS headers for fast incremental
// ripple detection by the pipeline.
package csharpanalyser
