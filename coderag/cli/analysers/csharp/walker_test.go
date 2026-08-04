package csharpanalyser

import (
	"os"
	"path/filepath"
	"strings"
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

func parseMethodContent(t *testing.T, src string, seed ...func(*GlobalIndex)) *FileResult {
	t.Helper()
	tmpFile := filepath.Join(t.TempDir(), "test.cs")
	if err := os.WriteFile(tmpFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	index := NewGlobalIndex()
	for _, s := range seed {
		s(index)
	}
	result, err := ParseFile(tmpFile, index)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func findMethod(t *testing.T, r *FileResult) Entity {
	t.Helper()
	for _, e := range r.Entities {
		if e.Type == EntityMethod {
			return e
		}
	}
	t.Fatal("no METHOD entity found")
	return Entity{}
}

func findRelation(t *testing.T, r *FileResult, source, target, keywords string) Relation {
	t.Helper()
	for _, rel := range r.Relations {
		if rel.Source == source && rel.Target == target && rel.Keywords == keywords {
			return rel
		}
	}
	t.Fatalf("no relation found: source=%q target=%q keywords=%q", source, target, keywords)
	return Relation{}
}

func TestParseMethodWithOutKeyword(t *testing.T) {
	r := parseMethodContent(t,
		"class C {\n\tbool TryParse(string s, out int result) { return true; }\n}\n",
	)
	m := findMethod(t, r)
	if !strings.Contains(m.Description, "out int result") {
		t.Errorf("description should contain 'out int result', got: %q", m.Description)
	}
	if !strings.Contains(m.Description, "TryParse(string s, out int result)") {
		t.Errorf("description should contain 'TryParse(string s, out int result)', got: %q", m.Description)
	}
}

func TestParseMethodWithRefKeyword(t *testing.T) {
	r := parseMethodContent(t,
		"class C {\n\tvoid Swap(ref int a, ref int b) { }\n}\n",
	)
	m := findMethod(t, r)
	if !strings.Contains(m.Description, "ref int a, ref int b") {
		t.Errorf("description should contain 'ref int a, ref int b', got: %q", m.Description)
	}
	if !strings.Contains(m.Description, "Swap(ref int a, ref int b)") {
		t.Errorf("description should contain 'Swap(ref int a, ref int b)', got: %q", m.Description)
	}
}

func TestParseMethodWithParamsKeyword(t *testing.T) {
	r := parseMethodContent(t,
		"class C {\n\tvoid Log(params string[] msgs) { }\n}\n",
	)
	m := findMethod(t, r)
	if !strings.Contains(m.Description, "params string[] msgs") {
		t.Errorf("description should contain 'params string[] msgs', got: %q", m.Description)
	}
}

func TestParseMethodWithLambdaDelegateType(t *testing.T) {
	r := parseMethodContent(t,
		"class C {\n\tvoid Process(Func<int, bool> predicate) { }\n}\n",
	)
	m := findMethod(t, r)
	if !strings.Contains(m.Description, "Func<int, bool> predicate") {
		t.Errorf("description should contain 'Func<int, bool> predicate', got: %q", m.Description)
	}
}

func TestParseMethodWithTupleType(t *testing.T) {
	parts := splitParams("(int, int) coords")
	if len(parts) != 1 || parts[0] != "(int, int) coords" {
		t.Errorf("splitParams must keep nested parens intact; got %v", parts)
	}
}

func TestParseMethodWithAttributeOnParam(t *testing.T) {
	r := parseMethodContent(t,
		"class C {\n\tvoid Foo([NotNull] string s) { }\n}\n",
	)
	m := findMethod(t, r)
	if !strings.Contains(m.Description, "[NotNull] string s") {
		t.Errorf("description should contain '[NotNull] string s', got: %q", m.Description)
	}
}

func TestParseMethodWithDefaultValue(t *testing.T) {
	r := parseMethodContent(t,
		"class C {\n\tvoid Bar(int x = 5, string label = \"hi\") { }\n}\n",
	)
	m := findMethod(t, r)
	if !strings.Contains(m.Description, "int x = 5, string label = \"hi\"") {
		t.Errorf("description should contain 'int x = 5, string label = \"hi\"', got: %q", m.Description)
	}
}

func TestParseMethodNoParams(t *testing.T) {
	r := parseMethodContent(t,
		"class C {\n\tvoid Baz() { }\n}\n",
	)
	m := findMethod(t, r)
	if !strings.Contains(m.Description, "C.Baz() -> RETURNS -> void") {
		t.Errorf("description should contain 'C.Baz() -> RETURNS -> void', got: %q", m.Description)
	}
}

func TestParseMethodRelationsWithNamespace(t *testing.T) {
	src := "namespace TestApp {\n\tclass Order { }\n\tclass OrderService {\n\t\tpublic void PlaceOrder(Order order) { }\n\t\tpublic decimal Total(Order order) { return 0; }\n\t}\n}\n"
	r := parseMethodContent(t, src,
		func(i *GlobalIndex) {
			i.Classes["TestApp.Order"] = &ClassInfo{Name: "TestApp.Order"}
		},
	)
	for _, e := range r.Entities {
		if e.Type != EntityMethod {
			continue
		}
		if !strings.Contains(e.Description, "(") || !strings.Contains(e.Description, ")") {
			t.Errorf("method %q description should include parens, got %q", e.Name, e.Description)
		}
	}
	count := 0
	for _, rel := range r.Relations {
		if rel.Source == "TestApp.OrderService.PlaceOrder" &&
			rel.Target == "TestApp.Order" &&
			rel.Keywords == "depends_on" {
			if !strings.Contains(rel.Description, "PlaceOrder(Order order)") {
				t.Errorf("depends_on description should contain 'PlaceOrder(Order order)', got %q", rel.Description)
			}
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 depends_on relation PlaceOrder->Order, got %d", count)
	}
}

func TestSplitParamsBalanced(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"Order order", []string{"Order order"}},
		{"Order order, int qty", []string{"Order order", "int qty"}},
		{"Func<int, Order> callback", []string{"Func<int, Order> callback"}},
		{"Dictionary<string, List<int>> data", []string{"Dictionary<string, List<int>> data"}},
		{"(int, int) coords", []string{"(int, int) coords"}},
		{"(int, int) coords, int qty", []string{"(int, int) coords", "int qty"}},
		{"ref int a, ref int b", []string{"ref int a", "ref int b"}},
	}
	for _, tc := range cases {
		got := splitParams(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("splitParams(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitParams(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestParseParamDecl(t *testing.T) {
	cases := []struct {
		in       string
		wantType string
		wantName string
		wantOK   bool
	}{
		{"Order order", "Order", "order", true},
		{"out int result", "int", "result", true},
		{"ref int a", "int", "a", true},
		{"params string[] messages", "string[]", "messages", true},
		{"Func<int, Order> callback", "Func<int, Order>", "callback", true},
		{"Dictionary<string, List<int>> data", "Dictionary<string, List<int>>", "data", true},
		{"(int, int) coords", "(int, int)", "coords", true},
		{"[NotNull] string s", "string", "s", true},
		{"int x = 5", "int", "x", true},
		{`string s = "hi"`, "string", "s", true},
		{"Order? order", "Order?", "order", true},
		{"List<string> items", "List<string>", "items", true},
		{"int[,] grid", "int[,]", "grid", true},
		{"Action a = null", "Action", "a", true},
		{"Action<int> h = x => Log(x)", "Action<int>", "h", true},
		{"this StringBuilder sb", "StringBuilder", "sb", true},
		{"", "", "", false},
		{"int", "", "", false},
		{"   ", "", "", false},
	}
	for _, tc := range cases {
		gotType, gotName, gotOK := parseParamDecl(tc.in)
		if gotOK != tc.wantOK || gotType != tc.wantType || gotName != tc.wantName {
			t.Errorf("parseParamDecl(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.in, gotType, gotName, gotOK, tc.wantType, tc.wantName, tc.wantOK)
		}
	}
}

func TestParseComplexMethodsFixture(t *testing.T) {
	fixturePath := filepath.Join("testdata", "TestApp", "ComplexMethods.cs")
	if _, err := os.Stat(fixturePath); err != nil {
		t.Skipf("fixture %s not present: %v", fixturePath, err)
	}
	index := NewGlobalIndex()
	index.Classes["TestApp.Order"] = &ClassInfo{Name: "TestApp.Order"}
	r, err := ParseFile(fixturePath, index)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]string{
		"TestApp.ComplexMethods.TryParse":  "out int result",
		"TestApp.ComplexMethods.Swap":      "ref int a, ref int b",
		"TestApp.ComplexMethods.Log":       "params string[] messages",
		"TestApp.ComplexMethods.Process":   "Func<int, bool> predicate, int seed",
		"TestApp.ComplexMethods.Send":      "int priority = 0, bool ccSender = true",
		"TestApp.ComplexMethods.Save":      "[NotNull] string key, [NotNull] string value",
		"TestApp.ComplexMethods.ExtendTo":  "this string s, int times",
		"TestApp.ComplexMethods.GetAsync":  "CancellationToken ct = default",
	}
	for _, e := range r.Entities {
		if e.Type != EntityMethod {
			continue
		}
		want, ok := expected[e.Name]
		if !ok {
			continue
		}
		if !strings.Contains(e.Description, want) {
			t.Errorf("method %q description should contain %q, got %q",
				e.Name, want, e.Description)
		}
	}

	for _, rel := range r.Relations {
		if rel.Keywords != "method, defines" {
			continue
		}
		if !strings.HasPrefix(rel.Source, "TestApp.ComplexMethods.") {
			continue
		}
		if rel.Source == "TestApp.ComplexMethods" {
			continue
		}
		if !strings.Contains(rel.Description, "(") || !strings.Contains(rel.Description, ")") {
			t.Errorf("HAS_METHOD relation %s -> %s should carry the signature, got %q",
				rel.Source, rel.Target, rel.Description)
		}
	}
}
