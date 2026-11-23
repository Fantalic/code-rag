package ziganalyser

import (
	"fmt"
)

// EntityType is the discriminator written into the first column of an
// entity row in the output file (e.g. FUNCTION, STRUCT, ENUM).
type EntityType string

const (
	EntityFunction   EntityType = "FUNCTION"
	EntityStruct     EntityType = "STRUCT"
	EntityEnum       EntityType = "ENUM"
	EntityEnumMember EntityType = "ENUM_MEMBER"
	EntityUnion      EntityType = "UNION"
	EntityOpaque     EntityType = "OPAQUE"
	EntityErrorSet   EntityType = "ERROR_SET"
	EntityConst      EntityType = "CONST"
	EntityVar        EntityType = "VAR"
	EntityTest       EntityType = "TEST"
	EntityExports    EntityType = "EXPORTS"
	// File-scoped metadata entities — emitted alongside the rest so the
	// downstream parser picks them up uniformly.
	EntityImport EntityType = "IMPORT"
	EntityUsing  EntityType = "USING"
)

// Entity is one named declaration extracted from a Zig file.
type Entity struct {
	Name        string
	Type        EntityType
	Description string
	FilePath    string
	LineNo      int // 1-based start line
	EndLine     int // 1-based inclusive end line
	Visibility  string // "pub" | "" (private / unknown)
}

// Relation is one edge between two entities within or across files.
// Keywords follow the same convention as the C# analyser: lowercase
// comma-separated tag plus the verb, e.g. "method, defines".
type Relation struct {
	Source      string
	Target      string
	Description string
	Keywords    string
	Weight      float64
}

// FunctionInfo describes one callable discovered during the index phase.
type FunctionInfo struct {
	Name     string
	Owner    string // enclosing struct / "" for free functions
	FilePath string
	LineNo   int
}

// FieldInfo describes one field of a struct/union.
type FieldInfo struct {
	Name     string
	Owner    string
	FilePath string
	LineNo   int
}

// TypeInfo describes one named type declaration.
type TypeInfo struct {
	Kind     string // struct | enum | union | opaque | error_set
	FilePath string
	LineNo   int
}

// GlobalIndex holds every symbol known to the project so call sites can
// be resolved to their declaring file. Built once per scan in parallel
// from the input file list.
type GlobalIndex struct {
	Functions map[string][]*FunctionInfo
	Fields    map[string][]*FieldInfo
	Types     map[string]*TypeInfo
	// SimpleNames maps "Foo" -> ["file_a.Foo", "file_b.Foo"] for fallback
	// resolution when no namespace/using qualifies a call site.
	SimpleNames map[string][]string
}

func NewGlobalIndex() *GlobalIndex {
	return &GlobalIndex{
		Functions:   make(map[string][]*FunctionInfo),
		Fields:      make(map[string][]*FieldInfo),
		Types:       make(map[string]*TypeInfo),
		SimpleNames: make(map[string][]string),
	}
}

// CallSite records one place where a function is invoked. CalleeName is
// the fully-qualified callee (e.g. "OrderService.highestId" or a free
// function name) so the emitted relation target matches a real entity
// node. ResolvedFile holds the callee's declaring file; it is populated
// only when the GlobalIndex could match the call, otherwise the call is
// dropped from output to avoid phantom UNKNOWN nodes.
type CallSite struct {
	CallerFile   string
	CallerMethod string
	CalleeName   string
	CalleeOwner  string // enclosing type of the callee, if known ("")
	LineNo       int
	ResolvedFile string
}

// FileResult is everything the analyser extracted from one .zig file.
// Drives both the on-disk output (.txt) and the scan/update diff.
type FileResult struct {
	FilePath  string
	Entities  []Entity
	Relations []Relation
	Calls     []CallSite
	Exports   []string // visible (pub) symbols other files might import
	Imports   []string // symbols referenced from other files / std
}

// DiffReport captures what changed between two file results so the
// pipeline can schedule re-analysis of importers.
type DiffReport struct {
	AddedSymbols   []string
	RemovedSymbols []string
}

func (d *DiffReport) IsEmpty() bool {
	return len(d.AddedSymbols) == 0 && len(d.RemovedSymbols) == 0
}

func (d *DiffReport) String() string {
	var s string
	for _, sym := range d.RemovedSymbols {
		s += fmt.Sprintf("REMOVED_SYMBOL: %s\n", sym)
	}
	for _, sym := range d.AddedSymbols {
		s += fmt.Sprintf("ADDED_SYMBOL:   %s\n", sym)
	}
	return s
}
