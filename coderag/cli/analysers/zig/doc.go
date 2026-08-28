// Package ziganalyser implements a Go-based Zig static analyser that
// extracts entities (functions, structs, enums, error sets, constants,
// variables, tests) and relationships (calls, fields, imports, member
// membership) from Zig codebases.
//
// It outputs structured text files (one per .zig file) in the same
// # FORMAT: 2 contract as the C# analyser. The pipeline reads the
// # EXPORTS: / # IMPORTS: headers for fast incremental ripple detection
// when a single file changes.
package ziganalyser
