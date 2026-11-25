package parser

import (
	"testing"
)

func TestParsePythonFile_BasicEntities(t *testing.T) {
	content := `# FORMAT: 2
CLASS|MyClass|/path/to/mod.py|1|20|A sample class
FUNCTION|my_func|/path/to/mod.py|22|30|A sample function
METHOD|my_method|/path/to/mod.py|32|45|A sample method
PROPERTY|my_prop|/path/to/mod.py|47|50|A sample property
CONST|MY_CONST|/path/to/mod.py|52|52|A constant
VAR|my_var|/path/to/mod.py|54|54|A variable
IMPORT|os|/path/to/mod.py|0|0|Import os
IMPORT|sys|/path/to/mod.py|0|0|Import sys
`
	entities, relations := ParsePythonFile("/path/to/mod.py", content)

	if len(entities) != 8 {
		t.Fatalf("expected 8 entities, got %d", len(entities))
	}

	// Check entity types
	wantTypes := map[string]string{
		"MyClass":   "CLASS",
		"my_func":   "FUNCTION",
		"my_method": "METHOD",
		"my_prop":   "PROPERTY",
		"MY_CONST":  "CONST",
		"my_var":    "VAR",
		"os":        "IMPORT",
		"sys":       "IMPORT",
	}
	for _, e := range entities {
		if want, ok := wantTypes[e.Name]; !ok {
			t.Errorf("unexpected entity name: %s", e.Name)
		} else if e.EntityType != want {
			t.Errorf("entity %s: expected type %s, got %s", e.Name, want, e.EntityType)
		}
	}

	if len(relations) != 0 {
		t.Fatalf("expected 0 relations, got %d", len(relations))
	}
}

func TestParsePythonFile_RelationAndCall(t *testing.T) {
	content := `CLASS|MyClass|/path/to/mod.py|1|20|A sample class
RELATION|MyClass|my_method|MyClass defines my_method|method, defines|/path/to/mod.py|1.0
RELATION|MyModule|my_func|MyModule uses my_func|uses|/path/to/mod.py|0.8
CALL|/path/to/caller.py|caller_func|callee_func|Owner|42
`
	entities, relations := ParsePythonFile("/path/to/mod.py", content)

	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	if entities[0].Name != "MyClass" {
		t.Errorf("expected entity name MyClass, got %s", entities[0].Name)
	}

	if len(relations) != 3 {
		t.Fatalf("expected 3 relations, got %d", len(relations))
	}

	// Check RELATION rows — each source is distinct so map lookup is safe.
	relSrcs := map[string]string{}
	for _, r := range relations {
		relSrcs[r.Source] = r.Target
	}
	if tgt, ok := relSrcs["MyClass"]; !ok || tgt != "my_method" {
		t.Errorf("expected MyClass -> my_method relation, got %v", relSrcs)
	}
	if tgt, ok := relSrcs["MyModule"]; !ok || tgt != "my_func" {
		t.Errorf("expected MyModule -> my_func relation, got %v", relSrcs)
	}
	// Verify CALL row parsed as relation
	var callFound bool
	for _, r := range relations {
		if r.Keywords == "calls" {
			callFound = true
			if r.Source != "caller_func" || r.Target != "callee_func" {
				t.Errorf("CALL row: expected caller_func -> callee_func, got %s -> %s", r.Source, r.Target)
			}
		}
	}
	if !callFound {
		t.Error("expected a CALL-derived relation with keywords=calls")
	}
}

func TestParsePythonFile_Sidecars(t *testing.T) {
	content := `# FORMAT: 2
COVERAGE|project-key|{"covered": 10, "total": 20}
CHANGELOG|/path/to/mod.py|{"timestamp": 1234567890, "changed": true}
CLASS|MyClass|/path/to/mod.py|1|20|A sample class
`
	entities, _ := ParsePythonFile("/path/to/mod.py", content)

	if len(entities) != 3 {
		t.Fatalf("expected 3 entities (1 CLASS + 2 sidecars), got %d", len(entities))
	}

	// Check COVERAGE entity
	var covFound, changelogFound bool
	for _, e := range entities {
		if e.EntityType == "COVERAGE" && e.Name == "COVERAGE:project-key" {
			covFound = true
		}
		if e.EntityType == "CHANGELOG" && e.Name == "CHANGELOG:/path/to/mod.py" {
			changelogFound = true
		}
	}
	if !covFound {
		t.Error("expected COVERAGE entity")
	}
	if !changelogFound {
		t.Error("expected CHANGELOG entity")
	}
}

func TestParsePythonFile_DedupEntities(t *testing.T) {
	content := `CLASS|MyClass|/path/to/mod.py|1|20|First declaration
CLASS|MyClass|/path/to/other.py|1|20|Second declaration
`
	entities, _ := ParsePythonFile("/path/to/mod.py", content)

	if len(entities) != 1 {
		t.Fatalf("expected 1 deduplicated entity, got %d", len(entities))
	}
	if entities[0].Name != "MyClass" {
		t.Errorf("expected entity name MyClass, got %s", entities[0].Name)
	}
}

func TestParsePythonFile_DedupRelations(t *testing.T) {
	content := `RELATION|MyClass|my_method|MyClass defines my_method|method, defines|/path/to/mod.py|1.0
RELATION|MyClass|my_method|MyClass defines my_method|method, defines|/path/to/other.py|1.0
`
	_, relations := ParsePythonFile("/path/to/mod.py", content)

	if len(relations) != 1 {
		t.Fatalf("expected 1 deduplicated relation, got %d", len(relations))
	}
}

func TestParsePythonOutput_Merge(t *testing.T) {
	results := []struct {
		Filename string
		Content  string
	}{
		{
			Filename: "/path/to/mod.py",
			Content: `CLASS|MyClass|/path/to/mod.py|1|20|First file
FUNCTION|my_func|/path/to/mod.py|22|30|From mod.py
`,
		},
		{
			Filename: "/path/to/other.py",
			Content: `CLASS|MyClass|/path/to/other.py|1|20|Second file
CONST|MY_CONST|/path/to/other.py|32|32|A constant
`,
		},
	}

	entities, relations := ParsePythonOutput(results)

	// Should have 3 unique entities: MyClass (from mod.py, preferred by basename), my_func, MY_CONST
	if len(entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(entities))
	}

	// Verify MyClass was chosen from mod.py (basename matches)
	var myClass Entity
	for _, e := range entities {
		if e.Name == "MyClass" {
			myClass = e
			break
		}
	}
	if myClass.FilePath != "/path/to/mod.py" {
		t.Errorf("expected MyClass from mod.py, got %s", myClass.FilePath)
	}

	if len(relations) != 0 {
		t.Fatalf("expected 0 relations, got %d", len(relations))
	}
}

func TestParsePythonFile_BlankAndComments(t *testing.T) {
	content := `
# FORMAT: 2

CLASS|MyClass|/path/to/mod.py|1|20|A class

# Another comment
FUNCTION|my_func|/path/to/mod.py|22|30|A function
`
	entities, _ := ParsePythonFile("/path/to/mod.py", content)

	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
}

func TestParsePythonFile_ExportsHeader(t *testing.T) {
	content := `# FORMAT: 2
# EXPORTS: _require_admin, mode
# MTIME: 1794356321
# a normal comment
FUNCTION|my_func|/path/to/mod.py|1|10|A function
`
	entities, _ := ParsePythonFile("/path/to/mod.py.txt", content)

	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}

	// Find the EXPORTS entity
	var expEntity *Entity
	for i := range entities {
		if entities[i].EntityType == "EXPORTS" {
			expEntity = &entities[i]
			break
		}
	}
	if expEntity == nil {
		t.Fatal("expected an EXPORTS entity")
	}
	if expEntity.Name != "/path/to/mod.py:EXPORTS" {
		t.Errorf("EXPORTS entity Name: expected /path/to/mod.py:EXPORTS, got %s", expEntity.Name)
	}
	if expEntity.Description != "_require_admin, mode [MTIME=1794356321]" {
		t.Errorf("EXPORTS entity Description: expected _require_admin, mode [MTIME=1794356321], got %s", expEntity.Description)
	}
	if expEntity.FilePath != "/path/to/mod.py" {
		t.Errorf("EXPORTS entity FilePath: expected /path/to/mod.py, got %s", expEntity.FilePath)
	}
	if expEntity.SourceID != "/path/to/mod.py.txt" {
		t.Errorf("EXPORTS entity SourceID: expected /path/to/mod.py.txt, got %s", expEntity.SourceID)
	}
	if expEntity.StartLine != 1 || expEntity.EndLine != 1 {
		t.Errorf("EXPORTS entity StartLine/EndLine: expected 1/1, got %d/%d", expEntity.StartLine, expEntity.EndLine)
	}

	// Verify the normal # comment produced no entity (only 2 entities total).
}

func TestParsePythonFile_EmptyExportsHeader(t *testing.T) {
	content := `# EXPORTS:
FUNCTION|f|/path/to/mod.py|1|2|desc
`
	entities, _ := ParsePythonFile("/path/to/mod.py.txt", content)

	if len(entities) != 1 {
		t.Fatalf("expected 1 entity (no EXPORTS sidecar), got %d", len(entities))
	}
	for _, e := range entities {
		if e.EntityType == "EXPORTS" {
			t.Error("unexpected EXPORTS entity for empty header")
		}
	}
}
