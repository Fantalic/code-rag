package parser

import (
	"testing"
)

func TestParseCSharpFile(t *testing.T) {
	content := `# EXPORTS: Person, Calculator, Calculator.Add
# IMPORTS: System, int, string
CLASS|Person|/path/Person.cs|5|class Person
PROPERTY|Name|/path/Person.cs|7|Person.Name -> RETURNS -> string
METHOD|GetName|/path/Person.cs|10|Person.GetName() -> RETURNS -> string
RELATION|Person|Name|Person.Name|property, has|/path/Person.cs|0.8
RELATION|Person|GetName|Person.GetName()|method, defines|/path/Person.cs|1.0
`

	entities, relations := ParseCSharpFile("Person.cs.txt", content)

	if len(entities) != 3 {
		t.Errorf("expected 3 entities, got %d", len(entities))
	}
	if len(relations) != 2 {
		t.Errorf("expected 2 relations, got %d", len(relations))
	}

	// Check CLASS entity
	foundClass := false
	for _, e := range entities {
		if e.Name == "Person" && e.EntityType == "CLASS" {
			foundClass = true
			if len(e.Methods) != 1 || e.Methods[0] != "GetName" {
				t.Errorf("class Methods = %v, expected [GetName]", e.Methods)
			}
		}
	}
	if !foundClass {
		t.Error("CLASS entity not found")
	}
}

func TestParseCSharpFileEmpty(t *testing.T) {
	content := `# EXPORTS: 
# IMPORTS: 
`
	entities, relations := ParseCSharpFile("empty.txt", content)
	if len(entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(entities))
	}
	if len(relations) != 0 {
		t.Errorf("expected 0 relations, got %d", len(relations))
	}
}
