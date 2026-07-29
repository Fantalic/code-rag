package csharpanalyser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseClassEntity(t *testing.T) {
	content := []byte("class MyClass {\n}\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(result.Entities))
	}
	e := result.Entities[0]
	if e.Name != "MyClass" {
		t.Errorf("expected Name=MyClass, got %q", e.Name)
	}
	if e.Type != EntityClass {
		t.Errorf("expected Type=EntityClass, got %v", e.Type)
	}
	if e.FilePath != tmpFile {
		t.Errorf("expected FilePath=%s, got %s", tmpFile, e.FilePath)
	}
	if e.LineNo != 1 {
		t.Errorf("expected LineNo=1, got %d", e.LineNo)
	}
}

func TestParseClassWithMethod(t *testing.T) {
	content := []byte("class MyClass {\n\tvoid Foo() { }\n}\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(result.Entities))
	}
	var hasClass, hasMethod bool
	for _, e := range result.Entities {
		switch e.Name {
		case "MyClass":
			hasClass = true
			if e.Type != EntityClass {
				t.Errorf("class entity type: got %v, want EntityClass", e.Type)
			}
		case "MyClass.Foo":
			hasMethod = true
			if e.Type != EntityMethod {
				t.Errorf("method entity type: got %v, want EntityMethod", e.Type)
			}
		}
	}
	if !hasClass {
		t.Error("missing class entity MyClass")
	}
	if !hasMethod {
		t.Error("missing method entity MyClass.Foo")
	}
	if len(result.Relations) < 1 {
		t.Fatal("expected at least 1 relation")
	}
	rel := result.Relations[0]
	if rel.Source != "MyClass" || rel.Target != "MyClass.Foo" {
		t.Errorf("expected relation MyClass->MyClass.Foo, got %s->%s", rel.Source, rel.Target)
	}
}

func TestParseClassWithNamespace(t *testing.T) {
	content := []byte("namespace MyNs {\nclass MyClass {\n}\n}\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(result.Entities))
	}
	e := result.Entities[0]
	if e.Name != "MyNs.MyClass" {
		t.Errorf("expected Name=MyNs.MyClass, got %q", e.Name)
	}
}

func TestParseImplementsRelation(t *testing.T) {
	content := []byte("class MyClass : IMyInterface {\n}\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	index.Classes["IMyInterface"] = &ClassInfo{Name: "IMyInterface"}
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(result.Entities))
	}
	found := false
	for _, r := range result.Relations {
		if r.Source == "MyClass" && r.Target == "IMyInterface" && r.Keywords == "implements" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected implements relation from MyClass to IMyInterface")
	}
}

func TestParseExtendsRelation(t *testing.T) {
	content := []byte("class MyClass : BaseClass {\n}\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	index.Classes["BaseClass"] = &ClassInfo{Name: "BaseClass"}
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(result.Entities))
	}
	found := false
	for _, r := range result.Relations {
		if r.Source == "MyClass" && r.Target == "BaseClass" && r.Keywords == "inheritance, extends" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected extends relation from MyClass to BaseClass")
	}
}

func TestParsePropertyEntity(t *testing.T) {
	content := []byte("class MyClass {\n\tint MyProp { get; set; }\n}\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range result.Entities {
		if e.Name == "MyClass.MyProp" && e.Type == EntityProperty {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected property entity MyClass.MyProp")
	}
}

func TestParseAutoProperty(t *testing.T) {
	content := []byte("class MyClass {\n\tpublic int MyProp { get; set; }\n}\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range result.Entities {
		if e.Name == "MyClass.MyProp" && e.Type == EntityProperty {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auto-property entity MyClass.MyProp")
	}
}

func TestParseMultipleClasses(t *testing.T) {
	content := []byte("class A {\n}\nclass B {\n}\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(result.Entities))
	}
	names := make(map[string]bool)
	for _, e := range result.Entities {
		names[e.Name] = true
	}
	if !names["A"] {
		t.Error("missing entity A")
	}
	if !names["B"] {
		t.Error("missing entity B")
	}
}

func TestParseEmptyFile(t *testing.T) {
	content := []byte("")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(result.Entities))
	}
}

func TestParseCommentsOnly(t *testing.T) {
	content := []byte("// this is a comment\n// another comment\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(result.Entities))
	}
}

func TestParseNestedClass(t *testing.T) {
	content := []byte("class Outer {\n\tclass Inner {\n\t}\n}\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(result.Entities))
	}
	names := make(map[string]EntityType)
	for _, e := range result.Entities {
		names[e.Name] = e.Type
	}
	if names["Outer"] != EntityClass {
		t.Error("missing or wrong type for Outer")
	}
	if names["Outer.Inner"] != EntityClass {
		t.Error("missing or wrong type for Outer.Inner")
	}
}

func TestParseCalls(t *testing.T) {
	content := []byte("class MyClass {\n\tvoid Foo() {\n\t\tOther obj = null;\n\t\tobj.DoSomething();\n\t}\n}\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Calls) < 1 {
		t.Fatal("expected at least 1 call site")
	}
	cs := result.Calls[0]
	if cs.CalleeName != "Other.DoSomething" {
		t.Errorf("expected CalleeName=Other.DoSomething, got %q", cs.CalleeName)
	}
	if cs.CalleeClass != "Other" {
		t.Errorf("expected CalleeClass=Other, got %q", cs.CalleeClass)
	}
	if cs.CallerMethod != "MyClass.Foo" {
		t.Errorf("expected CallerMethod=MyClass.Foo, got %q", cs.CallerMethod)
	}
}

func TestParseWithGlobalUsings(t *testing.T) {
	content := []byte("global using System;\nclass MyClass { }\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.GlobalUsings) != 1 {
		t.Fatalf("expected 1 global using, got %d", len(result.GlobalUsings))
	}
	if result.GlobalUsings[0] != "System" {
		t.Errorf("expected global using System, got %q", result.GlobalUsings[0])
	}
}

func TestParseWithUsings(t *testing.T) {
	content := []byte("using System;\nusing System.Collections.Generic;\nclass MyClass { }\n")
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Imports) < 2 {
		t.Fatalf("expected at least 2 imports, got %d", len(result.Imports))
	}
	hasSystem := false
	hasGeneric := false
	for _, imp := range result.Imports {
		if imp == "System" {
			hasSystem = true
		}
		if imp == "System.Collections.Generic" {
			hasGeneric = true
		}
	}
	if !hasSystem {
		t.Error("missing import System")
	}
	if !hasGeneric {
		t.Error("missing import System.Collections.Generic")
	}
}

func TestFileNameOnly(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"test.cs", "test"},
		{"path/to/file.cs", "file"},
		{"noext", "noext"},
		{"/absolute/path/MyClass.cs", "MyClass"},
		{"", "."},
	}
	for _, tt := range tests {
		got := FileNameOnly(tt.path)
		if got != tt.want {
			t.Errorf("FileNameOnly(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestReadSourceFile(t *testing.T) {
	content := "hello world"
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSourceFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != content {
		t.Errorf("ReadSourceFile = %q, want %q", got, content)
	}
}
