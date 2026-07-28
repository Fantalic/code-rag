package csharpanalyser

import (
	"strings"
	"sync"
)

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

type IndexBuilder struct {
	mu     sync.Mutex
	index  *GlobalIndex
}

func NewIndexBuilder() *IndexBuilder {
	return &IndexBuilder{
		index: NewGlobalIndex(),
	}
}

func (ib *IndexBuilder) BuildIndex(files []string) (*GlobalIndex, error) {
	var wg sync.WaitGroup
	type result struct {
		file  string
		fr    *FileResult
		err   error
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

	index := NewGlobalIndex()

	for r := range ch {
		if r.err != nil {
			continue
		}
		ib.mu.Lock()
		for _, e := range r.fr.Entities {
			switch e.Type {
			case EntityClass, EntityInterface, EntityStruct, EntityEnum:
				ci := &ClassInfo{
					Name:       e.Name,
					FilePath:   e.FilePath,
					LineNo:     e.LineNo,
				}
				index.Classes[e.Name] = ci
			case EntityMethod:
				mi := &MethodInfo{
					Name:     e.Name,
					FilePath: e.FilePath,
					LineNo:   e.LineNo,
				}
				index.Methods[e.Name] = append(index.Methods[e.Name], mi)
			case EntityProperty:
				pi := &PropertyInfo{
					Name:     e.Name,
					FilePath: e.FilePath,
					LineNo:   e.LineNo,
				}
				index.Properties[e.Name] = append(index.Properties[e.Name], pi)
			}
		}
		for _, gu := range r.fr.GlobalUsings {
			if !containsString(index.GlobalUsings, gu) {
				index.GlobalUsings = append(index.GlobalUsings, gu)
			}
		}
		ib.mu.Unlock()
	}

	// Build simple-name -> fully-qualified-names lookup
	for name := range index.Classes {
		simple := name
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			simple = name[idx+1:]
		}
		index.SimpleNames[simple] = append(index.SimpleNames[simple], name)
	}

	ib.index = index
	return index, nil
}

func (ib *IndexBuilder) GetIndex() *GlobalIndex {
	ib.mu.Lock()
	defer ib.mu.Unlock()
	return ib.index
}

func ResolveCallSite(call CallSite, index *GlobalIndex) (resolvedFile string, resolvedMethod string) {
	if call.CalleeClass != "" {
		if cls, ok := index.Classes[call.CalleeClass]; ok {
			resolvedFile = cls.FilePath
		}
	}

	if methods, ok := index.Methods[call.CalleeName]; ok && len(methods) > 0 {
		for _, m := range methods {
			if resolvedFile == "" || m.FilePath == resolvedFile {
				return m.FilePath, m.Name
			}
		}
		return methods[0].FilePath, methods[0].Name
	}

	return "", ""
}

func BuildInheritanceRelations(index *GlobalIndex) []Relation {
	var relations []Relation
	for _, cls := range index.Classes {
		if cls.BaseType != "" {
			if _, ok := index.Classes[cls.BaseType]; ok {
				relations = append(relations, Relation{
					Source:      cls.Name,
					Target:      cls.BaseType,
					Description: cls.Name + " extends " + cls.BaseType,
					Keywords:    "inheritance, extends",
					Weight:      1.0,
				})
			}
		}
	}
	return relations
}

func FindExportedSymbols(index *GlobalIndex, filePath string) []string {
	var symbols []string
	for _, cls := range index.Classes {
		if cls.FilePath == filePath {
			symbols = append(symbols, cls.Name)
		}
	}
	for name, methods := range index.Methods {
		for _, m := range methods {
			if m.FilePath == filePath {
				symbols = append(symbols, name)
			}
		}
	}
	return symbols
}
