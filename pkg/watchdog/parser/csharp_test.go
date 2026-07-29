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

	// Verify Relation fields are populated correctly
	for _, rel := range relations {
		if rel.SourceID != "Person.cs.txt" {
			t.Errorf("relation SourceID = %q, expected %q", rel.SourceID, "Person.cs.txt")
		}
		if rel.FilePath != "/path/Person.cs" {
			t.Errorf("relation FilePath = %q, expected %q", rel.FilePath, "/path/Person.cs")
		}
	}

	// Verify first relation details
	if relations[0].Source != "Person" || relations[0].Target != "Name" {
		t.Errorf("relation[0] = %s -> %s, expected Person -> Name", relations[0].Source, relations[0].Target)
	}
	if relations[0].Keywords != "property, has" {
		t.Errorf("relation[0].Keywords = %q, expected %q", relations[0].Keywords, "property, has")
	}
	if relations[0].Weight != 0.8 {
		t.Errorf("relation[0].Weight = %f, expected 0.8", relations[0].Weight)
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

func TestParseRelationLineEdgeCases(t *testing.T) {
	t.Run("missing file path defaults to empty string", func(t *testing.T) {
		// The RELATION format requires 7 parts, so a line with only 6 parts should fail
		line := "RELATION|A|B|desc|keyword"
		if _, ok := parseRelationLine(line); ok {
			t.Error("expected parseRelationLine to fail with < 7 parts")
		}
	})

	t.Run("invalid weight uses default 1.0", func(t *testing.T) {
		line := "RELATION|A|B|desc|kw|/path/file.cs|invalid"
		rel, ok := parseRelationLine(line)
		if !ok {
			t.Fatal("expected valid parse")
		}
		if rel.Weight != 1.0 {
			t.Errorf("expected weight 1.0, got %f", rel.Weight)
		}
	})

	t.Run("empty source fails", func(t *testing.T) {
		line := "RELATION||B|desc|kw|/path/file.cs|1.0"
		if _, ok := parseRelationLine(line); ok {
			t.Error("expected parseRelationLine to fail with empty source")
		}
	})

	t.Run("empty target fails", func(t *testing.T) {
		line := "RELATION|A||desc|kw|/path/file.cs|1.0"
		if _, ok := parseRelationLine(line); ok {
			t.Error("expected parseRelationLine to fail with empty target")
		}
	})

	t.Run("CALL line produces correct relation", func(t *testing.T) {
		line := "CALL|/path/Caller.cs|CallerMethod|CalleeName|CalleeClass|42"
		rel, ok := parseCallLine(line)
		if !ok {
			t.Fatal("expected valid CALL parse")
		}
		if rel.Source != "CallerMethod" {
			t.Errorf("expected Source=CallerMethod, got %q", rel.Source)
		}
		if rel.Target != "CalleeName" {
			t.Errorf("expected Target=CalleeName, got %q", rel.Target)
		}
		if rel.Keywords != "calls" {
			t.Errorf("expected Keywords=calls, got %q", rel.Keywords)
		}
		if rel.Weight != 0.5 {
			t.Errorf("expected Weight=0.5, got %f", rel.Weight)
		}
		// CALL lines do NOT set SourceID/FilePath (format has no file path field)
		if rel.SourceID != "" {
			t.Errorf("expected empty SourceID for CALL, got %q", rel.SourceID)
		}
		if rel.FilePath != "" {
			t.Errorf("expected empty FilePath for CALL, got %q", rel.FilePath)
		}
	})
}

func TestParseCSharpOutputDedup(t *testing.T) {
	results := []struct {
		Filename string
		Content  string
	}{
		{
			Filename: "FileA.cs.txt",
			Content: "CLASS|MyClass|/path/FileA.cs|10|MyClass from A\n" +
				"RELATION|MyClass|IService|MyClass implements IService|implements|/path/FileA.cs|1.0\n",
		},
		{
			Filename: "FileB.cs.txt",
			Content: "CLASS|MyClass|/path/FileB.cs|5|MyClass from B (redefinition)\n" +
				"RELATION|MyClass|IOther|MyClass implements IOther|implements|/path/FileB.cs|0.9\n",
		},
	}

	entities, relations := ParseCSharpOutput(results)

	// MyClass should be deduplicated — only one entity
	if len(entities) != 1 {
		t.Fatalf("expected 1 deduplicated entity, got %d", len(entities))
	}
	if entities[0].Name != "MyClass" {
		t.Errorf("expected entity name MyClass, got %q", entities[0].Name)
	}

	// Both relations should be present (different targets = different keys)
	if len(relations) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(relations))
	}

	// Each relation should have SourceID/FilePath from its respective file
	for _, rel := range relations {
		if rel.SourceID == "" {
			t.Errorf("relation %s->%s has empty SourceID", rel.Source, rel.Target)
		}
		if rel.FilePath == "" {
			t.Errorf("relation %s->%s has empty FilePath", rel.Source, rel.Target)
		}
	}
}

func TestParseCSharpOutputDedupRelations(t *testing.T) {
	results := []struct {
		Filename string
		Content  string
	}{
		{
			Filename: "FileA.cs.txt",
			Content: "CLASS|A|/path/A.cs|1|class A\n" +
				"RELATION|A|B|A depends on B|depends_on|/path/A.cs|1.0\n",
		},
		{
			Filename: "FileB.cs.txt",
			Content: "CLASS|B|/path/B.cs|1|class B\n" +
				"RELATION|A|B|A depends on B (duplicate)|depends_on|/path/B.cs|1.0\n",
		},
	}

	_, relations := ParseCSharpOutput(results)

	// Same (source, target, keywords) should be deduplicated
	if len(relations) != 1 {
		t.Fatalf("expected 1 deduplicated relation, got %d", len(relations))
	}
}
