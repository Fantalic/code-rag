package parser

import (
	"strings"
	"testing"
)

// TestParseZigFile verifies the parser accepts the same line formats
// the analyser writes and surfaces every entity/relation type.
func TestParseZigFile(t *testing.T) {
	sample := `# FORMAT: 2
# EXPORTS: Order, OrderError
# IMPORTS: std
FUNCTION|totalPrice|src/order.zig|30|32|fn totalPrice [pub]
STRUCT|Order|src/order.zig|22|27|struct [pub]
ENUM|Status|src/order.zig|8|14|enum [pub]
ERROR_SET|OrderError|src/order.zig|16|20|error set
CONST|Order.id|src/order.zig|23|23|field id
ENUM_MEMBER|Status.pending|src/order.zig|9|9|Status.pending
RELATION|Status|Status.pending|Status.pending|member, has|src/order.zig|0.7
RELATION|Order|Order.id|Order has field id|field, defines|src/order.zig|0.7
CALL|src/main.zig|main|place|svc|25
IMPORT|import:std|src/main.zig|5|5|@import("std")
TEST|test:"simple"|src/order.zig|40|43|test "simple"
COVERAGE|ZigApp|{"files":4,"warnings":"-"}
CHANGELOG|src/order.zig|{"file":"src/order.zig","action":"added","symbol":"X","changed_at":1}
`

	entities, relations := ParseZigFile("src/order.zig", sample)

	wantEntityTypes := map[string]bool{
		"FUNCTION": false, "STRUCT": false, "ENUM": false,
		"ERROR_SET": false, "CONST": false, "ENUM_MEMBER": false,
		"IMPORT": false, "TEST": false, "COVERAGE": false,
		"CHANGELOG": false,
	}
	for _, e := range entities {
		if _, ok := wantEntityTypes[e.EntityType]; ok {
			wantEntityTypes[e.EntityType] = true
		}
	}
	for typ, seen := range wantEntityTypes {
		if !seen {
			t.Errorf("expected to find entity type %s in output", typ)
		}
	}

	// One RELATION row should produce a "calls" keyword relation from
	// the CALL row; verify keywords round-trip.
	callsFound := 0
	memberFound := 0
	fieldFound := 0
	for _, r := range relations {
		switch r.Keywords {
		case "calls":
			callsFound++
			if r.Source != "main" || r.Target != "place" {
				t.Errorf("unexpected call source/target: %s -> %s", r.Source, r.Target)
			}
		case "member, has":
			memberFound++
		case "field, defines":
			fieldFound++
		}
	}
	if callsFound != 1 {
		t.Errorf("expected 1 call relation, got %d", callsFound)
	}
	if memberFound != 1 {
		t.Errorf("expected 1 member relation, got %d", memberFound)
	}
	if fieldFound != 1 {
		t.Errorf("expected 1 field relation, got %d", fieldFound)
	}
}

// TestParseZigOutputDedup checks the merge step picks the
// basename==name candidate when the same entity appears in multiple
// files.
func TestParseZigOutputDedup(t *testing.T) {
	// Two files, both declaring a "Point" struct with different paths.
	// The merge should prefer the one whose file basename == name.
	results := []struct {
		Filename string
		Content  string
	}{
		{
			Filename: "src/main.zig",
			Content: strings.Join([]string{
				"# FORMAT: 2",
				"# EXPORTS: ",
				"# IMPORTS: ",
				`STRUCT|Point|src/main.zig|1|1|struct`,
			}, "\n"),
		},
		{
			Filename: "src/Point.zig",
			Content: strings.Join([]string{
				"# FORMAT: 2",
				"# EXPORTS: ",
				"# IMPORTS: ",
				`STRUCT|Point|src/Point.zig|1|1|struct`,
			}, "\n"),
		},
	}

	entities, _ := ParseZigOutput(results)
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity after dedup, got %d", len(entities))
	}
	if !strings.HasSuffix(entities[0].FilePath, "Point.zig") {
		t.Errorf("expected dedup to prefer Point.zig, got %s", entities[0].FilePath)
	}
}
