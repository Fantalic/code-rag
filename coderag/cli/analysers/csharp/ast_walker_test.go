package csharpanalyser

// Tests for the tree-sitter walker: constructs the old regex scanner could
// not see (local functions, enum members, constructors, fields), source
// spans, and noise immunity.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseSource(t *testing.T, src string) *FileResult {
	return parseSourceWithIndex(t, src, nil)
}

func parseSourceWithIndex(t *testing.T, src string, index *GlobalIndex) *FileResult {
	t.Helper()
	if index == nil {
		index = NewGlobalIndex()
	}
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func findEntityByType(r *FileResult, et EntityType) []Entity {
	var out []Entity
	for _, e := range r.Entities {
		if e.Type == et {
			out = append(out, e)
		}
	}
	return out
}

func TestLocalFunctionsExtracted(t *testing.T) {
	r := parseSource(t, `namespace App {
public class Snake {
	public void Run() {
		int GetDirection() { return 1; }
		void PositionFood() { Move(); }
		GetDirection();
	}
	private void Move() { }
}
}
`)
	methods := map[string]bool{}
	for _, e := range findEntityByType(r, EntityMethod) {
		methods[e.Name] = true
	}
	if !methods["App.Snake.Run.GetDirection"] {
		t.Errorf("local function GetDirection missing; methods=%v", methods)
	}
	if !methods["App.Snake.Run.PositionFood"] {
		t.Errorf("local function PositionFood missing; methods=%v", methods)
	}
	// Defining relation points at the ENCLOSING METHOD, not the class.
	found := false
	for _, rel := range r.Relations {
		if rel.Source == "App.Snake.Run" && rel.Target == "App.Snake.Run.GetDirection" && rel.Keywords == "method, defines" {
			found = true
		}
	}
	if !found {
		t.Error("missing method-defines edge Run -> Run.GetDirection")
	}
}

func TestEnumMembersExtracted(t *testing.T) {
	r := parseSource(t, `namespace App {
public enum Direction { Up, Down = 2, Left, Right }
}
`)
	members := findEntityByType(r, EntityEnumMember)
	if len(members) != 4 {
		t.Fatalf("want 4 enum members, got %d: %v", len(members), members)
	}
	names := map[string]int{}
	for i, m := range members {
		names[m.Name] = i
	}
	if _, ok := names["App.Direction.Up"]; !ok {
		t.Error("member Up missing")
	}
	if _, ok := names["App.Direction.Down"]; !ok {
		t.Error("member Down missing")
	}
	// Ordering must be preserved so Direction.Up vs Down is answerable.
	if members[0].Name != "App.Direction.Up" || members[1].Name != "App.Direction.Down" {
		t.Errorf("member order wrong: %s then %s", members[0].Name, members[1].Name)
	}
}

func TestConstructorsAndFieldsExtracted(t *testing.T) {
	r := parseSource(t, `namespace App {
public class Player {
	private int health;
	public string Name { get; set; }
	public Player() {
		string s = "}";
		if (health < 0) { health = 0; }
	}
}
}
`)
	ctors := findEntityByType(r, EntityConstructor)
	if len(ctors) != 1 || ctors[0].Name != "App.Player.Player" {
		t.Fatalf("constructor entity wrong: %v", ctors)
	}
	fields := findEntityByType(r, EntityField)
	if len(fields) != 1 || fields[0].Name != "App.Player.health" {
		t.Fatalf("field entities wrong: %v", fields)
	}
	// The literal "}" inside a string must not desync parsing: the ctor
	// body statement produced no entities.
	for _, e := range r.Entities {
		switch e.Type {
		case EntityMethod, EntityProperty, EntityField:
			if strings.HasSuffix(e.Name, ".if") || strings.HasSuffix(e.Name, ".health = 0") ||
				strings.HasSuffix(e.Name, ".s") {
				t.Errorf("noise entity from ctor body leaked: %q (%s)", e.Name, e.Type)
			}
		}
	}
}

func TestControlFlowNeverBecomesEntity(t *testing.T) {
	r := parseSource(t, `namespace App {
class MazeClass {
	void Walk(int I, int J) {
		while (I > 0) { I--; }
		if (J == 3) { return; }
		foreach (var x in new int[3]) { }
		static void NotReal(int a, int b) { }
	}
}
}
`)
	for _, e := range r.Entities {
		suffixes := []string{".if", ".while", ".foreach", ".return", ".static"}
		for _, sfx := range suffixes {
			if strings.HasSuffix(e.Name, sfx) {
				t.Errorf("noise entity %q (%s)", e.Name, e.Type)
			}
		}
	}
	// The nested local function IS real and must exist.
	foundLocal := false
	for _, m := range findEntityByType(r, EntityMethod) {
		if strings.HasSuffix(m.Name, "Walk.NotReal") {
			foundLocal = true
		}
	}
	if !foundLocal {
		t.Error("local function NotReal should be indexed as METHOD")
	}
}

func TestEndLineSpansOnMethodBody(t *testing.T) {
	src := "namespace App {\nclass C {\n\tpublic void M() {\n\t\tvar a = 1;\n\t\tvar b = 2;\n\t}\n}\n}\n"
	// lines:      1            2          3                    4         5       6  7
	r := parseSource(t, src)
	m := findMethod(t, r)
	if m.Name != "App.C.M" {
		t.Fatalf("wrong method %q", m.Name)
	}
	if m.LineNo != 3 {
		t.Errorf("start line = %d, want 3", m.LineNo)
	}
	if m.EndLine != 6 {
		t.Errorf("end line = %d, want 6", m.EndLine)
	}
	cls := findEntityByType(r, EntityClass)
	if len(cls) != 1 || cls[0].LineNo != 2 || cls[0].EndLine != 7 {
		t.Errorf("class span = [%d,%d], want [2,7]", cls[0].LineNo, cls[0].EndLine)
	}
}

func TestCallsResolvedThroughLocalsAndParams(t *testing.T) {
	index := NewGlobalIndex()
	index.Classes["TestApp.Order"] = &ClassInfo{Name: "TestApp.Order"}
	r := parseSourceWithIndex(t, `namespace TestApp {
class Service {
	void Handle(Order order) {
		order.PlaceOrder();
		var other = order;
		other.Cancel();
		Console.WriteLine("x");
	}
}
}
`, index)
	var calls []CallSite
	for _, c := range r.Calls {
		calls = append(calls, c)
	}
	got := map[string]string{}
	for _, c := range calls {
		got[c.CalleeName] = c.CalleeClass
	}
	if got["TestApp.Order.PlaceOrder"] != "TestApp.Order" {
		t.Errorf("param-typed call unresolved: %v", got)
	}
	if got["TestApp.Order.Cancel"] != "TestApp.Order" {
		t.Errorf("local-var-aliased call unresolved: %v", got)
	}
	// Console.WriteLine has no resolvable receiver type and must be skipped.
	if _, ok := got["Console.WriteLine"]; ok {
		t.Errorf("unresolvable BCL call should not produce CallSite: %v", got)
	}
}

func TestFileScopedNamespace(t *testing.T) {
	r := parseSource(t, `namespace App.Sub;
public class Foo {
	public void Bar() { }
}
`)
	found := false
	for _, c := range findEntityByType(r, EntityClass) {
		if c.Name == "App.Sub.Foo" {
			found = true
		}
	}
	if !found {
		t.Error("file-scoped namespace not applied to qualified name")
	}
}
