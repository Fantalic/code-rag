package ziganalyser

// Global symbol index. Built once per scan from every .zig file in the
// project, then handed to ParseFile so call sites and field accesses
// can be resolved against the project-wide symbol table. Mirrors the
// C# analyser's index.go at a smaller scale: no semantic model, just
// a name -> file map.

import (
	"strings"
	"sync"
)

// IndexBuilder walks every input file in parallel (parsing only the
// names it needs, not full call resolution), then aggregates the
// symbol table.
type IndexBuilder struct {
	mu    sync.Mutex
	index *GlobalIndex
}

func NewIndexBuilder() *IndexBuilder {
	return &IndexBuilder{
		index: NewGlobalIndex(),
	}
}

// BuildIndex parses every file in `files` once with a nil cross-file
// index (so it doesn't recursively try to resolve), collects the
// declarations, and returns the merged GlobalIndex.
func (ib *IndexBuilder) BuildIndex(files []string) (*GlobalIndex, error) {
	var wg sync.WaitGroup
	type result struct {
		file string
		fr   *FileResult
		err  error
	}
	ch := make(chan result, len(files))

	for _, f := range files {
		wg.Add(1)
		go func(fp string) {
			defer wg.Done()
			fr, err := ParseFile(fp, nil)
			ch <- result{file: fp, fr: fr, err: err}
		}(f)
	}

	wg.Wait()
	close(ch)

	for r := range ch {
		if r.err != nil || r.fr == nil {
			continue
		}
		ib.mu.Lock()
		ib.merge(r.fr)
		ib.mu.Unlock()
	}

	// Populate SimpleNames from the fully-qualified set.
	for name := range ib.index.Functions {
		simple := name
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			simple = name[idx+1:]
		}
		ib.index.SimpleNames[simple] = append(ib.index.SimpleNames[simple], name)
	}
	for name := range ib.index.Fields {
		simple := name
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			simple = name[idx+1:]
		}
		ib.index.SimpleNames[simple] = append(ib.index.SimpleNames[simple], name)
	}
	for name := range ib.index.Types {
		simple := name
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			simple = name[idx+1:]
		}
		ib.index.SimpleNames[simple] = append(ib.index.SimpleNames[simple], name)
	}

	return ib.index, nil
}

// merge folds one FileResult's entities into the index. Called with
// ib.mu held.
func (ib *IndexBuilder) merge(fr *FileResult) {
	for _, e := range fr.Entities {
		switch e.Type {
		case EntityFunction:
			owner := ""
			if idx := strings.LastIndex(e.Name, "."); idx >= 0 {
				owner = e.Name[:idx]
			}
			ib.index.Functions[e.Name] = append(ib.index.Functions[e.Name], &FunctionInfo{
				Name:     e.Name,
				Owner:    owner,
				FilePath: e.FilePath,
				LineNo:   e.LineNo,
			})
		case EntityStruct, EntityEnum, EntityUnion, EntityOpaque, EntityErrorSet:
			ib.index.Types[e.Name] = &TypeInfo{
				Kind:     strings.ToLower(strings.TrimPrefix(string(e.Type), "ENTITY_")),
				FilePath: e.FilePath,
				LineNo:   e.LineNo,
			}
		case EntityConst, EntityEnumMember:
			// Treat struct fields and enum members uniformly as fields so
			// obj.field() lookups can match them.
			if idx := strings.LastIndex(e.Name, "."); idx >= 0 {
				owner := e.Name[:idx]
				ib.index.Fields[e.Name] = append(ib.index.Fields[e.Name], &FieldInfo{
					Name:     e.Name,
					Owner:    owner,
					FilePath: e.FilePath,
					LineNo:   e.LineNo,
				})
			}
		}
	}
}

// findContainer reads the project root for .zig files, skipping build
// caches and zig-out directories.
func CollectZigFiles(projectDir string) ([]string, error) {
	files, _, err := CollectZigFilesWithCoverage(projectDir)
	return files, err
}
