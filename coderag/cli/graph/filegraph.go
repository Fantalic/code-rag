// Package graph derives file-level views of the knowledge graph from its
// existing entity nodes and relation edges, without storing file nodes.
package graph

import (
	"path/filepath"
	"sort"
	"strings"
)

type FileStat struct {
	FilePath string
	Nodes    int
	FanIn    int
	FanOut   int
}

// FileGraph maps entities to their source files and derives cross-file
// reference edges from the relation graph.
type FileGraph struct {
	files        []string
	fileEntities map[string][]string
	entityFile   map[string]string
	refs         map[string][]string
	deps         map[string][]string
}

// Build derives a FileGraph from raw LightRAG graph nodes and edges. Nodes
// must carry an id/entity_id and a properties.file_path; edges carry
// source/target entity names (see EdgeEndpoints).
func Build(nodes []map[string]any, edges []map[string]any) *FileGraph {
	g := &FileGraph{
		fileEntities: make(map[string][]string),
		entityFile:   make(map[string]string),
		refs:         make(map[string][]string),
		deps:         make(map[string][]string),
	}

	for _, n := range nodes {
		id := NodeID(n)
		if id == "" {
			continue
		}
		fp := NodeFilePath(n)
		if fp == "" {
			continue
		}
		g.entityFile[id] = fp
		g.fileEntities[fp] = append(g.fileEntities[fp], id)
	}

	for _, e := range edges {
		src, tgt := EdgeEndpoints(e)
		fs, ok := g.entityFile[src]
		if !ok {
			continue
		}
		ft, ok := g.entityFile[tgt]
		if !ok {
			continue
		}
		if fs == ft {
			continue
		}
		g.refs[fs] = addUniqueSorted(g.refs[fs], ft)
		g.deps[ft] = addUniqueSorted(g.deps[ft], fs)
	}

	for f := range g.fileEntities {
		g.files = append(g.files, f)
	}
	sort.Strings(g.files)
	return g
}

// Files returns all files, sorted.
func (g *FileGraph) Files() []string {
	return g.files
}

// Refs returns the files directly referenced by the given file (out-edges).
func (g *FileGraph) Refs(file string) []string {
	return g.refs[file]
}

// Dependents returns the files that directly depend on the given file
// (in-edges).
func (g *FileGraph) Dependents(file string) []string {
	return g.deps[file]
}

// Entities returns the entity ids declared in the given file.
func (g *FileGraph) Entities(file string) []string {
	return g.fileEntities[file]
}

// ResolveFile matches a CLI argument to a known file. It prefers an exact
// path match, then a basename match, then a substring match.
func (g *FileGraph) ResolveFile(arg string) (string, bool) {
	if _, ok := g.fileEntities[arg]; ok {
		return arg, true
	}
	base := filepath.Base(arg)
	var contains []string
	for _, f := range g.files {
		if filepath.Base(f) == base {
			return f, true
		}
		if strings.Contains(f, arg) {
			contains = append(contains, f)
		}
	}
	if len(contains) == 1 {
		return contains[0], true
	}
	return "", false
}

// Stats returns per-file node and edge counts, sorted by path.
func (g *FileGraph) Stats() []FileStat {
	stats := make([]FileStat, 0, len(g.files))
	for _, f := range g.files {
		stats = append(stats, FileStat{
			FilePath: f,
			Nodes:    len(g.fileEntities[f]),
			FanIn:    len(g.deps[f]),
			FanOut:   len(g.refs[f]),
		})
	}
	return stats
}

// Cycles returns dependency cycles among files using Tarjan's strongly
// connected components. Each component with more than one file is a cycle;
// self-referencing files are also reported.
func (g *FileGraph) Cycles() [][]string {
	index := make(map[string]int)
	low := make(map[string]int)
	onStack := make(map[string]bool)
	var stack []string
	counter := 0
	var sccs [][]string

	var strongconnect func(v string)
	strongconnect = func(v string) {
		counter++
		index[v] = counter
		low[v] = counter
		stack = append(stack, v)
		onStack[v] = true

		for _, w := range g.refs[v] {
			if _, seen := index[w]; !seen {
				strongconnect(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if onStack[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}

		if low[v] == index[v] {
			var comp []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			sccs = append(sccs, comp)
		}
	}

	for _, f := range g.files {
		if _, seen := index[f]; !seen {
			strongconnect(f)
		}
	}

	var cycles [][]string
	for _, comp := range sccs {
		if len(comp) > 1 {
			sort.Strings(comp)
			cycles = append(cycles, comp)
		} else if containsString(g.refs[comp[0]], comp[0]) {
			cycles = append(cycles, comp)
		}
	}
	sort.Slice(cycles, func(i, j int) bool {
		return cycles[i][0] < cycles[j][0]
	})
	return cycles
}

// NodeID returns the entity id of a raw graph node.
func NodeID(node map[string]any) string {
	if id, ok := node["id"].(string); ok && id != "" {
		return id
	}
	if props, ok := node["properties"].(map[string]any); ok {
		if id, ok := props["entity_id"].(string); ok && id != "" {
			return id
		}
	}
	return ""
}

// NodeFilePath returns the source file of a raw graph node.
func NodeFilePath(node map[string]any) string {
	if props, ok := node["properties"].(map[string]any); ok {
		if fp, ok := props["file_path"].(string); ok && fp != "" {
			return fp
		}
	}
	return ""
}

// EdgeEndpoints returns the true source/target entity names of a raw graph
// edge. LightRAG swaps source/target lexicographically on read-back, so the
// original direction is read from properties.source/target when present.
func EdgeEndpoints(edge map[string]any) (string, string) {
	if props, ok := edge["properties"].(map[string]any); ok {
		s, sok := props["source"].(string)
		t, tok := props["target"].(string)
		if sok && tok && s != "" && t != "" {
			return s, t
		}
	}
	s, _ := edge["source"].(string)
	t, _ := edge["target"].(string)
	return s, t
}

func addUniqueSorted(slice []string, v string) []string {
	for _, existing := range slice {
		if existing == v {
			return slice
		}
	}
	slice = append(slice, v)
	sort.Strings(slice)
	return slice
}

func containsString(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}
