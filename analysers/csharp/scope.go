package csharpanalyser

import "path/filepath"

type ScopeTracker struct {
	fileIndex map[string]*FileResult
	globalIdx *GlobalIndex
}

func NewScopeTracker(idx *GlobalIndex) *ScopeTracker {
	return &ScopeTracker{
		fileIndex: make(map[string]*FileResult),
		globalIdx: idx,
	}
}

func (st *ScopeTracker) AddFileResult(fp string, fr *FileResult) {
	st.fileIndex[filepath.Clean(fp)] = fr
}

func (st *ScopeTracker) ResolveCalls(filePath string) []Relation {
	fr, ok := st.fileIndex[filepath.Clean(filePath)]
	if !ok {
		return nil
	}

	var relations []Relation
	for _, call := range fr.Calls {
		resolvedFile, resolvedMethod := ResolveCallSite(call, st.globalIdx)

		target := call.CalleeName
		if call.CalleeClass != "" {
			target = call.CalleeClass + "." + call.CalleeName
		}

		desc := call.CallerMethod + " calls " + target
		if resolvedFile != "" {
			desc = call.CallerMethod + " calls " + resolvedMethod + " in " + resolvedFile
		}

		rel := Relation{
			Source:      call.CallerMethod,
			Target:      target,
			Description: desc,
			Keywords:    "call",
			Weight:      0.5,
		}
		relations = append(relations, rel)

		if resolvedFile != "" {
			fr.Imports = append(fr.Imports, resolvedFile)
		}
	}

	return relations
}

func (st *ScopeTracker) ResolveAllCalls() map[string][]Relation {
	results := make(map[string][]Relation)
	for fp := range st.fileIndex {
		rels := st.ResolveCalls(fp)
		if len(rels) > 0 {
			results[fp] = rels
		}
	}
	return results
}
