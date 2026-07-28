package csharpanalyser

import "fmt"

type EntityType string

const (
	EntityClass     EntityType = "CLASS"
	EntityMethod    EntityType = "METHOD"
	EntityProperty  EntityType = "PROPERTY"
	EntityInterface EntityType = "INTERFACE"
	EntityStruct    EntityType = "STRUCT"
	EntityEnum      EntityType = "ENUM"
)

type Entity struct {
	Name        string
	Type        EntityType
	Description string
	FilePath    string
	LineNo      int
}

type Relation struct {
	Source      string
	Target      string
	Description string
	Keywords    string
	Weight      float64
}

type MethodInfo struct {
	Name       string
	ClassName  string
	ReturnType string
	Params     []ParamInfo
	FilePath   string
	LineNo     int
}

type ParamInfo struct {
	Name string
	Type string
}

type PropertyInfo struct {
	Name      string
	ClassName string
	Type      string
	FilePath  string
	LineNo    int
}

type ClassInfo struct {
	Name         string
	FilePath     string
	BaseType     string
	Interfaces   []string
	Methods      []string
	Properties   []string
	LineNo       int
}

type CallSite struct {
	CallerFile     string
	CallerMethod   string
	CalleeName     string
	CalleeClass    string
	LineNo         int
	ResolvedFile   string
	ResolvedMethod string
}

type TypeInfo struct {
	Kind     string   // class, interface, struct, enum
	FilePath string
	LineNo   int
}

type GlobalIndex struct {
	Classes    map[string]*ClassInfo
	Methods    map[string][]*MethodInfo
	Properties map[string][]*PropertyInfo
	Types      map[string]*TypeInfo
}

func NewGlobalIndex() *GlobalIndex {
	return &GlobalIndex{
		Classes:    make(map[string]*ClassInfo),
		Methods:    make(map[string][]*MethodInfo),
		Properties: make(map[string][]*PropertyInfo),
		Types:      make(map[string]*TypeInfo),
	}
}

type FileResult struct {
	FilePath  string
	Entities  []Entity
	Relations []Relation
	Exports   []string
	Imports   []string
	Calls     []CallSite
}

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
