package ziganalyser

// File-level orchestration for the Zig analyser. The actual AST traversal
// lives in ast_walker.go (tree-sitter-zig driven). This file keeps
// ParseFile, the per-file EXPORTS metadata emission, and the small
// helpers shared across the package.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// nowUnix returns the current unix timestamp. Wrapped so tests can
// override it later if they need to assert on sidecar rows.
var nowUnix = func() int64 { return time.Now().Unix() }

// Parser is the per-file state machine. One instance is created per
// ParseFile call; it accumulates entities, relations, calls, exports
// and imports as the tree-sitter walk progresses.
type Parser struct {
	sourcePath   string
	content      string
	contentBytes []byte

	entities  []Entity
	relations []Relation
	calls     []CallSite
	exports   []string
	imports   []string

	// Current container type name when walking struct/enum/union bodies.
	// Empty at file scope.
	currentType string
	// Most recent fn name we entered — used as the caller for call sites.
	currentFn string
	// isInsideTest counts how many test bodies the walker is currently
	// inside. Bumped while walking a test_declaration body so call sites
	// in tests don't generate phantom graph edges.
	isInsideTest int

	// Local scope tracker — pushed/popped as blocks open/close.
	scope *Scope

	index *GlobalIndex
}

// ParseFile reads the file at path, runs the tree-sitter walk, and
// returns the populated FileResult. The index may be nil for ad-hoc
// parses (used when callers don't need cross-file resolution).
func ParseFile(filePath string, index *GlobalIndex) (*FileResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	// Strip UTF-8 BOM (Zig source rarely has one but be defensive).
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	content := string(data)

	p := &Parser{
		sourcePath:   filePath,
		content:      content,
		contentBytes: data,
		index:        index,
		scope:        NewScope(nil),
	}

	p.parse()
	p.emitFileMetadata()
	p.dedupeAndSort()
	return &FileResult{
		FilePath:  filePath,
		Entities:  p.entities,
		Relations: p.relations,
		Calls:     p.calls,
		Exports:   p.exports,
		Imports:   p.imports,
	}, nil
}

// dedupeAndSort collapses duplicate exports/imports and orders them so
// the on-disk output is byte-stable across runs.
func (p *Parser) dedupeAndSort() {
	p.exports = dedupSorted(p.exports)
	p.imports = dedupSorted(p.imports)
}

func dedupSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// emitFileMetadata writes a single EXPORTS entity per source file
// carrying the full export list and mtime. Downstream MCP tools
// (dead_code) query this entity instead of re-reading analyser
// output files.
func (p *Parser) emitFileMetadata() {
	var mtime int64
	if info, err := os.Stat(p.sourcePath); err == nil {
		mtime = info.ModTime().Unix()
	}

	canonical := fileExportsEntityName(p.sourcePath)
	desc := strings.Join(p.exports, ", ")
	if mtime > 0 {
		desc = desc + " [MTIME=" + fmt.Sprint(mtime) + "]"
	}

	p.addEntity(Entity{
		Name:        canonical,
		Type:        EntityExports,
		Description: desc,
		FilePath:    p.sourcePath,
		LineNo:      1,
		EndLine:     1,
	})
}

// fileExportsEntityName is the stable canonical name for the per-file
// EXPORTS entity: "<basename-no-ext>:EXPORTS".
func fileExportsEntityName(filePath string) string {
	base := filepath.Base(filePath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return base + ":EXPORTS"
}

func (p *Parser) addEntity(e Entity) {
	p.entities = append(p.entities, e)
}

func (p *Parser) addRelation(r Relation) {
	p.relations = append(p.relations, r)
}

func (p *Parser) addCall(c CallSite) {
	p.calls = append(p.calls, c)
}
