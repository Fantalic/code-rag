package csharpanalyser

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempCSFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewIndexBuilder(t *testing.T) {
	ib := NewIndexBuilder()
	if ib == nil {
		t.Fatal("NewIndexBuilder() returned nil")
	}
	idx := ib.GetIndex()
	if idx == nil {
		t.Error("GetIndex() returned nil before BuildIndex")
	}
}

func TestBuildIndex(t *testing.T) {
	dir := t.TempDir()
	file1 := writeTempCSFile(t, dir, "test1.cs", `
namespace MyNamespace
{
	public class MyClass
	{
		public void MyMethod()
		{
		}
	}
}
`)
	file2 := writeTempCSFile(t, dir, "test2.cs", `
public class OtherClass
{
	public int MyProperty { get; set; }
}
`)
	ib := NewIndexBuilder()
	index, err := ib.BuildIndex([]string{file1, file2})
	if err != nil {
		t.Fatal(err)
	}
	if index == nil {
		t.Fatal("BuildIndex returned nil index")
	}

	if _, ok := index.Classes["MyNamespace.MyClass"]; !ok {
		t.Error("expected class MyNamespace.MyClass")
	}
	if _, ok := index.Classes["OtherClass"]; !ok {
		t.Error("expected class OtherClass")
	}

	methods, ok := index.Methods["MyNamespace.MyClass.MyMethod"]
	if !ok {
		t.Fatal("expected method MyNamespace.MyClass.MyMethod")
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 method entry, got %d", len(methods))
	}

	props, ok := index.Properties["OtherClass.MyProperty"]
	if !ok {
		t.Fatal("expected property OtherClass.MyProperty")
	}
	if len(props) != 1 {
		t.Fatalf("expected 1 property entry, got %d", len(props))
	}
}

func TestBuildIndexEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	file1 := writeTempCSFile(t, dir, "empty1.cs", "")
	file2 := writeTempCSFile(t, dir, "empty2.cs", "\n\n\n")
	file3 := writeTempCSFile(t, dir, "comments.cs", "// this is a comment\n// another comment\n")

	ib := NewIndexBuilder()
	index, err := ib.BuildIndex([]string{file1, file2, file3})
	if err != nil {
		t.Fatal(err)
	}
	if index == nil {
		t.Fatal("BuildIndex returned nil index")
	}
	if len(index.Classes) != 0 {
		t.Errorf("expected 0 classes, got %d", len(index.Classes))
	}
	if len(index.Methods) != 0 {
		t.Errorf("expected 0 methods, got %d", len(index.Methods))
	}
	if len(index.Properties) != 0 {
		t.Errorf("expected 0 properties, got %d", len(index.Properties))
	}
}

func TestGetIndex(t *testing.T) {
	dir := t.TempDir()
	file1 := writeTempCSFile(t, dir, "test.cs", `
namespace N
{
	class C
	{
	}
}
`)
	ib := NewIndexBuilder()
	built, err := ib.BuildIndex([]string{file1})
	if err != nil {
		t.Fatal(err)
	}
	idx := ib.GetIndex()
	if idx == nil {
		t.Fatal("GetIndex returned nil")
	}
	if idx != built {
		t.Error("GetIndex returned a different instance than BuildIndex")
	}
	if _, ok := idx.Classes["N.C"]; !ok {
		t.Error("expected class N.C in GetIndex result")
	}
}

func TestResolveCallSite(t *testing.T) {
	index := NewGlobalIndex()
	index.Classes["MyNamespace.MyClass"] = &ClassInfo{
		Name:     "MyNamespace.MyClass",
		FilePath: "test.cs",
	}
	index.Methods["MyNamespace.MyClass.MyMethod"] = []*MethodInfo{
		{Name: "MyNamespace.MyClass.MyMethod", FilePath: "test.cs"},
	}

	call := CallSite{
		CalleeClass: "MyNamespace.MyClass",
		CalleeName:  "MyNamespace.MyClass.MyMethod",
	}

	file, method := ResolveCallSite(call, index)
	if file != "test.cs" {
		t.Errorf("expected resolved file test.cs, got %s", file)
	}
	if method != "MyNamespace.MyClass.MyMethod" {
		t.Errorf("expected resolved method MyNamespace.MyClass.MyMethod, got %s", method)
	}
}

func TestResolveCallSiteNotFound(t *testing.T) {
	index := NewGlobalIndex()

	call := CallSite{
		CalleeClass: "NonExistent.Class",
		CalleeName:  "NonExistent.Class.Method",
	}

	file, method := ResolveCallSite(call, index)
	if file != "" {
		t.Errorf("expected empty file, got %s", file)
	}
	if method != "" {
		t.Errorf("expected empty method, got %s", method)
	}
}

func TestBuildInheritanceRelations(t *testing.T) {
	index := NewGlobalIndex()
	index.Classes["MyClass"] = &ClassInfo{
		Name:     "MyClass",
		FilePath: "a.cs",
		BaseType: "BaseClass",
	}
	index.Classes["BaseClass"] = &ClassInfo{
		Name:     "BaseClass",
		FilePath: "b.cs",
	}

	rels := BuildInheritanceRelations(index)
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rels))
	}
	if rels[0].Source != "MyClass" {
		t.Errorf("expected Source=MyClass, got %s", rels[0].Source)
	}
	if rels[0].Target != "BaseClass" {
		t.Errorf("expected Target=BaseClass, got %s", rels[0].Target)
	}
	if rels[0].Description != "MyClass extends BaseClass" {
		t.Errorf("expected 'MyClass extends BaseClass', got %s", rels[0].Description)
	}
	if rels[0].Weight != 1.0 {
		t.Errorf("expected Weight=1.0, got %f", rels[0].Weight)
	}
}

func TestBuildInheritanceRelationsNoBase(t *testing.T) {
	index := NewGlobalIndex()
	index.Classes["MyClass"] = &ClassInfo{
		Name:     "MyClass",
		FilePath: "a.cs",
	}

	rels := BuildInheritanceRelations(index)
	if len(rels) != 0 {
		t.Errorf("expected 0 relations, got %d", len(rels))
	}
}

func TestBuildInheritanceRelationsMissingBase(t *testing.T) {
	index := NewGlobalIndex()
	index.Classes["MyClass"] = &ClassInfo{
		Name:     "MyClass",
		FilePath: "a.cs",
		BaseType: "MissingBase",
	}

	rels := BuildInheritanceRelations(index)
	if len(rels) != 0 {
		t.Errorf("expected 0 relations, got %d", len(rels))
	}
}

func TestFindExportedSymbols(t *testing.T) {
	index := NewGlobalIndex()
	index.Classes["ClassA"] = &ClassInfo{
		Name:     "ClassA",
		FilePath: "file1.cs",
	}
	index.Classes["ClassB"] = &ClassInfo{
		Name:     "ClassB",
		FilePath: "file2.cs",
	}
	index.Methods["ClassA.MethodA"] = []*MethodInfo{
		{Name: "ClassA.MethodA", FilePath: "file1.cs"},
	}
	index.Methods["ClassB.MethodB"] = []*MethodInfo{
		{Name: "ClassB.MethodB", FilePath: "file2.cs"},
	}

	symbols := FindExportedSymbols(index, "file1.cs")
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols for file1.cs, got %d: %v", len(symbols), symbols)
	}

	found := make(map[string]bool)
	for _, s := range symbols {
		found[s] = true
	}
	if !found["ClassA"] {
		t.Error("expected ClassA in exported symbols")
	}
	if !found["ClassA.MethodA"] {
		t.Error("expected ClassA.MethodA in exported symbols")
	}
	if found["ClassB"] {
		t.Error("ClassB should not be in exported symbols for file1.cs")
	}
	if found["ClassB.MethodB"] {
		t.Error("ClassB.MethodB should not be in exported symbols for file1.cs")
	}
}
