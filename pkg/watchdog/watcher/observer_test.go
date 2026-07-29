package watcher

import (
	"testing"

	"github.com/coderag/pkg/watchdog/crud"
)

func TestFindAffectedFilesFromData(t *testing.T) {
	t.Run("identifies files that reference deleted entities", func(t *testing.T) {
		relations := []crud.RelationEdge{
			{Source: "DeletedClass", Target: "UserService", Keywords: "depends_on", SourceID: "/repo/DeletedClass.cs", FilePath: "/repo/DeletedClass.cs"},
			{Source: "MyClass", Target: "DeletedClass", Keywords: "calls", SourceID: "/repo/DeletedClass.cs", FilePath: "/repo/DeletedClass.cs"},
			{Source: "MyClass", Target: "UserService", Keywords: "depends_on", SourceID: "/repo/UserService.cs", FilePath: "/repo/UserService.cs"},
		}
		entityMap := map[string]string{
			"UserService":  "/repo/UserService.cs",
			"MyClass":      "/repo/MyClass.cs",
			"DeletedClass": "/repo/DeletedClass.cs",
		}

		affected := findAffectedFilesFromData(relations, entityMap, []string{"DeletedClass"}, "/repo/DeletedClass.cs")

		expected := map[string]bool{"/repo/UserService.cs": true, "/repo/MyClass.cs": true}
		if len(affected) != 2 {
			t.Fatalf("expected 2 affected files, got %d: %v", len(affected), affected)
		}
		for _, fp := range affected {
			if !expected[fp] {
				t.Errorf("unexpected affected file: %s", fp)
			}
		}
	})

	t.Run("returns empty when no relations reference deleted entities", func(t *testing.T) {
		relations := []crud.RelationEdge{
			{Source: "SomeClass", Target: "OtherClass", Keywords: "depends_on", SourceID: "/repo/SomeClass.cs", FilePath: "/repo/SomeClass.cs"},
		}
		entityMap := map[string]string{
			"IsolatedClass": "/repo/IsolatedClass.cs",
			"SomeClass":     "/repo/SomeClass.cs",
			"OtherClass":    "/repo/OtherClass.cs",
		}

		affected := findAffectedFilesFromData(relations, entityMap, []string{"IsolatedClass"}, "/repo/IsolatedClass.cs")

		if len(affected) != 0 {
			t.Errorf("expected 0 affected files, got %d: %v", len(affected), affected)
		}
	})

	t.Run("adds entity-map fallback when relation source_id is wrong", func(t *testing.T) {
		// Simulate relation stored with wrong source_id (before parser fix)
		relations := []crud.RelationEdge{
			{Source: "DeletedClass", Target: "UserService", Keywords: "depends_on", SourceID: "/repo/WrongFile.cs"},
		}
		entityMap := map[string]string{
			"UserService":  "/repo/UserService.cs",
			"DeletedClass": "/repo/DeletedClass.cs",
		}

		affected := findAffectedFilesFromData(relations, entityMap, []string{"DeletedClass"}, "/repo/DeletedClass.cs")

		// Both the wrong source_id and the correct entity-based path are returned
		if len(affected) != 2 {
			t.Fatalf("expected 2 affected files (source_id + entity fallback), got %d: %v", len(affected), affected)
		}
		hasCorrect := false
		for _, fp := range affected {
			if fp == "/repo/UserService.cs" {
				hasCorrect = true
			}
		}
		if !hasCorrect {
			t.Errorf("entity fallback /repo/UserService.cs not found in results: %v", affected)
		}
	})

	t.Run("excludes the deleted file path", func(t *testing.T) {
		relations := []crud.RelationEdge{
			{Source: "DeletedClass", Target: "UserService", Keywords: "depends_on", SourceID: "/repo/DeletedClass.cs"},
		}
		entityMap := map[string]string{
			"UserService":  "/repo/UserService.cs",
			"DeletedClass": "/repo/DeletedClass.cs",
		}

		affected := findAffectedFilesFromData(relations, entityMap, []string{"DeletedClass"}, "/repo/DeletedClass.cs")

		for _, fp := range affected {
			if fp == "/repo/DeletedClass.cs" {
				t.Errorf("excluded file path %q still in result", fp)
			}
		}
	})

	t.Run("both source_id and entity path added to set", func(t *testing.T) {
		// Both paths are additive — function doesn't "prefer" one over the other
		relations := []crud.RelationEdge{
			{Source: "DeletedClass", Target: "UserService", Keywords: "depends_on", SourceID: "/repo/UserService.cs"},
		}
		entityMap := map[string]string{
			"UserService":  "/repo/WrongEntityPath.cs",
			"DeletedClass": "/repo/DeletedClass.cs",
		}

		affected := findAffectedFilesFromData(relations, entityMap, []string{"DeletedClass"}, "/repo/DeletedClass.cs")

		if len(affected) != 2 {
			t.Fatalf("expected 2 (source_id + entity path), got %d: %v", len(affected), affected)
		}
	})
}
