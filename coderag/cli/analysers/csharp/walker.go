package csharpanalyser

// File-level orchestration for the C# analyser.
//
// Extraction itself lives in ast_walker.go (tree-sitter based). This file
// keeps ParseFile, type resolution against the GlobalIndex, per-file
// metadata emission, and small helpers shared across packages.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type classContext struct {
	name       string
	depth      int
	baseType   string
	interfaces []string
	lineNo     int
}

type Parser struct {
	sourcePath   string
	content      string
	contentBytes []byte

	entities  []Entity
	relations []Relation
	calls     []CallSite
	exports   []string
	imports   []string

	namespace    string
	usings       []string
	globalUsings []string

	// Namespace from a file-scoped namespace declaration; applies to all
	// subsequent top-level types because this grammar nests nothing under it.
	fileNamespace string

	index *GlobalIndex
}

func (p *Parser) resolveType(simpleName string) string {
	if p.index == nil {
		return simpleName
	}
	// 1. Current namespace
	if p.namespace != "" {
		candidate := p.namespace + "." + simpleName
		if _, ok := p.index.Classes[candidate]; ok {
			return candidate
		}
	}
	// 2. File-level usings
	for _, u := range p.usings {
		candidate := u + "." + simpleName
		if _, ok := p.index.Classes[candidate]; ok {
			return candidate
		}
	}
	// 3. Global usings (from index)
	for _, u := range p.index.GlobalUsings {
		candidate := u + "." + simpleName
		if _, ok := p.index.Classes[candidate]; ok {
			return candidate
		}
	}
	// 4. Exact match (already fully-qualified or global namespace)
	if _, ok := p.index.Classes[simpleName]; ok {
		return simpleName
	}
	// 5. Unique simple-name match across index (fallback)
	if matches, ok := p.index.SimpleNames[simpleName]; ok && len(matches) == 1 {
		return matches[0]
	}
	return simpleName
}

func ParseFile(filePath string, index *GlobalIndex) (*FileResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	// Strip UTF-8 BOM
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	content := string(data)
	p := &Parser{
		sourcePath:   filePath,
		content:      content,
		contentBytes: data,
		index:        index,
	}

	p.parse()
	p.emitFileMetadata()
	return &FileResult{
		FilePath:     filePath,
		Entities:     p.entities,
		Relations:    p.relations,
		Exports:      p.exports,
		Imports:      p.imports,
		Calls:        p.calls,
		GlobalUsings: p.globalUsings,
	}, nil
}

// emitFileMetadata emits one EXPORTS entity per source file, carrying the
// full export list and the file's mtime. Downstream MCP tools (dead_code)
// query this entity instead of re-reading analyser output files.
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

// fileExportsEntityName returns a stable canonical name for the
// per-file EXPORTS entity: "<basename-no-ext>:EXPORTS".
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

var keywords = map[string]bool{
	"if": true, "else": true, "for": true, "foreach": true, "while": true,
	"do": true, "switch": true, "return": true, "throw": true, "yield": true,
	"await": true, "new": true, "var": true, "base": true, "this": true,
	"in": true, "out": true, "ref": true, "sizeof": true, "typeof": true,
	"nameof": true, "true": true, "false": true, "null": true, "default": true,
}

func isKeyword(s string) bool {
	return keywords[s]
}

func ReadSourceFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func FileNameOnly(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".cs")
}
