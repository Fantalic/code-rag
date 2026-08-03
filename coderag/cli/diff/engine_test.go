package diff

import (
	"testing"
)

func TestDiffEntities(t *testing.T) {
	t.Run("NoChanges", func(t *testing.T) {
		old := map[string]map[string]any{
			"a": {"color": "red"},
			"b": {"color": "blue"},
		}
		new := map[string]map[string]any{
			"a": {"color": "red"},
			"b": {"color": "blue"},
		}
		actions := DiffEntities(old, new)
		if len(actions) != 0 {
			t.Errorf("expected 0 actions, got %d", len(actions))
		}
	})

	t.Run("EntityAdded", func(t *testing.T) {
		old := map[string]map[string]any{
			"a": {"color": "red"},
		}
		new := map[string]map[string]any{
			"a": {"color": "red"},
			"b": {"color": "blue"},
		}
		actions := DiffEntities(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Kind != ActionCreate {
			t.Errorf("expected CREATE, got %s", actions[0].Kind)
		}
		if actions[0].EntityName != "b" {
			t.Errorf("expected entity b, got %s", actions[0].EntityName)
		}
		if actions[0].Data["color"] != "blue" {
			t.Errorf("expected data color=blue, got %v", actions[0].Data["color"])
		}
	})

	t.Run("EntityDeleted", func(t *testing.T) {
		old := map[string]map[string]any{
			"a": {"color": "red"},
			"b": {"color": "blue"},
		}
		new := map[string]map[string]any{
			"a": {"color": "red"},
		}
		actions := DiffEntities(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Kind != ActionDelete {
			t.Errorf("expected DELETE, got %s", actions[0].Kind)
		}
		if actions[0].EntityName != "b" {
			t.Errorf("expected entity b, got %s", actions[0].EntityName)
		}
		if actions[0].Data == nil {
			t.Error("expected non-nil data map for delete")
		}
	})

	t.Run("EntityUpdated", func(t *testing.T) {
		old := map[string]map[string]any{
			"a": {"color": "red", "size": 10},
		}
		new := map[string]map[string]any{
			"a": {"color": "blue", "size": 10},
		}
		actions := DiffEntities(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Kind != ActionUpdate {
			t.Errorf("expected UPDATE, got %s", actions[0].Kind)
		}
		if actions[0].EntityName != "a" {
			t.Errorf("expected entity a, got %s", actions[0].EntityName)
		}
		if actions[0].Data["color"] != "blue" {
			t.Errorf("expected color=blue, got %v", actions[0].Data["color"])
		}
	})

	t.Run("Mixed", func(t *testing.T) {
		old := map[string]map[string]any{
			"keep":    {"v": 1},
			"delete":  {"v": 2},
			"update":  {"v": 3},
		}
		new := map[string]map[string]any{
			"keep":   {"v": 1},
			"update": {"v": 99},
			"add":    {"v": 4},
		}
		actions := DiffEntities(old, new)
		if len(actions) != 3 {
			t.Fatalf("expected 3 actions, got %d", len(actions))
		}
		kinds := map[string]ActionKind{}
		for _, a := range actions {
			kinds[a.EntityName] = a.Kind
		}
		if kinds["delete"] != ActionDelete {
			t.Errorf("expected delete=DELETE, got %s", kinds["delete"])
		}
		if kinds["add"] != ActionCreate {
			t.Errorf("expected add=CREATE, got %s", kinds["add"])
		}
		if kinds["update"] != ActionUpdate {
			t.Errorf("expected update=UPDATE, got %s", kinds["update"])
		}
	})

	t.Run("EmptyOld", func(t *testing.T) {
		old := map[string]map[string]any{}
		new := map[string]map[string]any{
			"a": {"x": 1},
		}
		actions := DiffEntities(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Kind != ActionCreate {
			t.Errorf("expected CREATE, got %s", actions[0].Kind)
		}
	})

	t.Run("EmptyNew", func(t *testing.T) {
		old := map[string]map[string]any{
			"a": {"x": 1},
		}
		new := map[string]map[string]any{}
		actions := DiffEntities(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Kind != ActionDelete {
			t.Errorf("expected DELETE, got %s", actions[0].Kind)
		}
	})

	t.Run("NilMaps", func(t *testing.T) {
		actions := DiffEntities(nil, nil)
		if len(actions) != 0 {
			t.Errorf("expected 0 actions for nil maps, got %d", len(actions))
		}
	})
}

func TestDiffRelations(t *testing.T) {
	t.Run("NoChanges", func(t *testing.T) {
		old := map[string]map[string]any{
			"a|b|kw": {"weight": 1.0},
		}
		new := map[string]map[string]any{
			"a|b|kw": {"weight": 1.0},
		}
		actions := DiffRelations(old, new)
		if len(actions) != 0 {
			t.Errorf("expected 0 actions, got %d", len(actions))
		}
	})

	t.Run("RelationAdded", func(t *testing.T) {
		old := map[string]map[string]any{}
		new := map[string]map[string]any{
			"x|y|rel": {"val": 42},
		}
		actions := DiffRelations(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Kind != ActionCreate {
			t.Errorf("expected CREATE, got %s", actions[0].Kind)
		}
		if actions[0].Source != "x" {
			t.Errorf("expected source x, got %s", actions[0].Source)
		}
		if actions[0].Target != "y" {
			t.Errorf("expected target y, got %s", actions[0].Target)
		}
		if actions[0].Keywords != "rel" {
			t.Errorf("expected keywords rel, got %s", actions[0].Keywords)
		}
	})

	t.Run("RelationDeleted", func(t *testing.T) {
		old := map[string]map[string]any{
			"x|y|rel": {"val": 42},
		}
		new := map[string]map[string]any{}
		actions := DiffRelations(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Kind != ActionDelete {
			t.Errorf("expected DELETE, got %s", actions[0].Kind)
		}
		if actions[0].Source != "x" {
			t.Errorf("expected source x, got %s", actions[0].Source)
		}
		if actions[0].Target != "y" {
			t.Errorf("expected target y, got %s", actions[0].Target)
		}
		if actions[0].Keywords != "rel" {
			t.Errorf("expected keywords rel, got %s", actions[0].Keywords)
		}
		if actions[0].Data == nil {
			t.Error("expected non-nil data map for delete")
		}
	})

	t.Run("RelationUpdated", func(t *testing.T) {
		old := map[string]map[string]any{
			"a|b|knows": {"strength": 0.5},
		}
		new := map[string]map[string]any{
			"a|b|knows": {"strength": 0.9},
		}
		actions := DiffRelations(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Kind != ActionUpdate {
			t.Errorf("expected UPDATE, got %s", actions[0].Kind)
		}
		if actions[0].Source != "a" {
			t.Errorf("expected source a, got %s", actions[0].Source)
		}
		if actions[0].Target != "b" {
			t.Errorf("expected target b, got %s", actions[0].Target)
		}
		if actions[0].Keywords != "knows" {
			t.Errorf("expected keywords knows, got %s", actions[0].Keywords)
		}
		if actions[0].Data["strength"] != 0.9 {
			t.Errorf("expected strength 0.9, got %v", actions[0].Data["strength"])
		}
	})

	t.Run("Mixed", func(t *testing.T) {
		old := map[string]map[string]any{
			"a|b|r1": {"x": 1},
			"a|b|r2": {"x": 2},
			"a|b|r3": {"x": 3},
		}
		new := map[string]map[string]any{
			"a|b|r2": {"x": 99},
			"a|b|r3": {"x": 3},
			"a|b|r4": {"x": 4},
		}
		actions := DiffRelations(old, new)
		if len(actions) != 3 {
			t.Fatalf("expected 3 actions, got %d", len(actions))
		}
		kinds := map[string]ActionKind{}
		for _, a := range actions {
			key := RelationKey(a.Source, a.Target, a.Keywords)
			kinds[key] = a.Kind
		}
		if kinds["a|b|r1"] != ActionDelete {
			t.Errorf("expected r1=DELETE, got %s", kinds["a|b|r1"])
		}
		if kinds["a|b|r2"] != ActionUpdate {
			t.Errorf("expected r2=UPDATE, got %s", kinds["a|b|r2"])
		}
		if kinds["a|b|r4"] != ActionCreate {
			t.Errorf("expected r4=CREATE, got %s", kinds["a|b|r4"])
		}
	})

	t.Run("EmptyOld", func(t *testing.T) {
		old := map[string]map[string]any{}
		new := map[string]map[string]any{
			"src|tgt|kw": {"v": 1},
		}
		actions := DiffRelations(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Kind != ActionCreate {
			t.Errorf("expected CREATE, got %s", actions[0].Kind)
		}
	})

	t.Run("EmptyNew", func(t *testing.T) {
		old := map[string]map[string]any{
			"src|tgt|kw": {"v": 1},
		}
		new := map[string]map[string]any{}
		actions := DiffRelations(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Kind != ActionDelete {
			t.Errorf("expected DELETE, got %s", actions[0].Kind)
		}
	})

	t.Run("RelationWithKeywords", func(t *testing.T) {
		old := map[string]map[string]any{
			"a|b|": {"v": 1},
		}
		new := map[string]map[string]any{
			"a|b|": {"v": 2},
		}
		actions := DiffRelations(old, new)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Source != "a" {
			t.Errorf("expected source a, got %s", actions[0].Source)
		}
		if actions[0].Target != "b" {
			t.Errorf("expected target b, got %s", actions[0].Target)
		}
		if actions[0].Keywords != "" {
			t.Errorf("expected empty keywords, got %s", actions[0].Keywords)
		}
	})
}

func TestDataChanged(t *testing.T) {
	t.Run("SameMaps", func(t *testing.T) {
		a := map[string]any{"color": "red", "size": 10}
		b := map[string]any{"color": "red", "size": 10}
		if dataChanged(a, b) {
			t.Error("expected false for identical maps")
		}
	})

	t.Run("Different", func(t *testing.T) {
		a := map[string]any{"color": "red"}
		b := map[string]any{"color": "blue"}
		if !dataChanged(a, b) {
			t.Error("expected true for different maps")
		}
	})

	t.Run("IgnoredCreatedAt", func(t *testing.T) {
		a := map[string]any{"color": "red", "created_at": "2024-01-01"}
		b := map[string]any{"color": "red", "created_at": "2025-01-01"}
		if dataChanged(a, b) {
			t.Error("expected false when only created_at differs")
		}
	})

	t.Run("SourceIDChangeDetected", func(t *testing.T) {
		a := map[string]any{"color": "red", "source_id": "s1"}
		b := map[string]any{"color": "red", "source_id": "s2"}
		if !dataChanged(a, b) {
			t.Error("expected true when only source_id differs")
		}
	})

	t.Run("IgnoredEntityID", func(t *testing.T) {
		a := map[string]any{"entity_id": "e1", "val": 1}
		b := map[string]any{"entity_id": "e2", "val": 1}
		if dataChanged(a, b) {
			t.Error("expected false when only entity_id differs")
		}
	})

	t.Run("IgnoredSrcID", func(t *testing.T) {
		a := map[string]any{"src_id": "x", "val": 1}
		b := map[string]any{"src_id": "y", "val": 1}
		if dataChanged(a, b) {
			t.Error("expected false when only src_id differs")
		}
	})

	t.Run("IgnoredTgtID", func(t *testing.T) {
		a := map[string]any{"tgt_id": "x", "val": 1}
		b := map[string]any{"tgt_id": "y", "val": 1}
		if dataChanged(a, b) {
			t.Error("expected false when only tgt_id differs")
		}
	})

	t.Run("NestedMap", func(t *testing.T) {
		a := map[string]any{"meta": map[string]any{"inner": 1}}
		b := map[string]any{"meta": map[string]any{"inner": 2}}
		if !dataChanged(a, b) {
			t.Error("expected true for nested maps with different values")
		}
	})

	t.Run("ArrayValues", func(t *testing.T) {
		a := map[string]any{"tags": []any{"x", "y"}}
		b := map[string]any{"tags": []any{"x", "z"}}
		if !dataChanged(a, b) {
			t.Error("expected true for different array values")
		}
	})
}

func TestEqualMaps(t *testing.T) {
	t.Run("Equal", func(t *testing.T) {
		a := map[string]any{"a": 1, "b": "hello"}
		b := map[string]any{"a": 1, "b": "hello"}
		if !equalMaps(a, b) {
			t.Error("expected true for equal maps")
		}
	})

	t.Run("DifferentValues", func(t *testing.T) {
		a := map[string]any{"a": 1}
		b := map[string]any{"a": 2}
		if equalMaps(a, b) {
			t.Error("expected false for different values")
		}
	})

	t.Run("DifferentLength", func(t *testing.T) {
		a := map[string]any{"a": 1, "b": 2}
		b := map[string]any{"a": 1}
		if equalMaps(a, b) {
			t.Error("expected false for different lengths")
		}
	})

	t.Run("NestedEqual", func(t *testing.T) {
		a := map[string]any{"inner": map[string]any{"x": 10}}
		b := map[string]any{"inner": map[string]any{"x": 10}}
		if !equalMaps(a, b) {
			t.Error("expected true for equal nested maps")
		}
	})
}

func TestEqualValues(t *testing.T) {
	t.Run("SameStrings", func(t *testing.T) {
		if !equalValues("foo", "foo") {
			t.Error("expected true for same strings")
		}
	})

	t.Run("DifferentStrings", func(t *testing.T) {
		if equalValues("foo", "bar") {
			t.Error("expected false for different strings")
		}
	})

	t.Run("SameNumbers", func(t *testing.T) {
		if !equalValues(42, 42) {
			t.Error("expected true for same integers")
		}
		if !equalValues(3.14, 3.14) {
			t.Error("expected true for same floats")
		}
	})

	t.Run("NestedMaps", func(t *testing.T) {
		a := map[string]any{"a": map[string]any{"b": 1}}
		b := map[string]any{"a": map[string]any{"b": 1}}
		if !equalValues(a, b) {
			t.Error("expected true for equal nested maps")
		}
	})

	t.Run("Arrays", func(t *testing.T) {
		a := []any{1, "two", 3.0}
		b := []any{1, "two", 3.0}
		if !equalValues(a, b) {
			t.Error("expected true for equal arrays")
		}
	})
}

func TestKeySet(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		m := map[string]map[string]any{
			"a": {"v": 1},
			"b": {"v": 2},
		}
		s := keySet(m)
		if len(s) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(s))
		}
		if !s["a"] {
			t.Error("expected key a")
		}
		if !s["b"] {
			t.Error("expected key b")
		}
	})

	t.Run("Empty", func(t *testing.T) {
		s := keySet(map[string]map[string]any{})
		if len(s) != 0 {
			t.Errorf("expected empty set, got %d", len(s))
		}
	})
}

func TestMapDiff(t *testing.T) {
	t.Run("Added", func(t *testing.T) {
		a := map[string]bool{"x": true}
		b := map[string]bool{"x": true, "y": true}
		res := mapDiff(a, b)
		if len(res.deleted) != 0 {
			t.Errorf("expected no deleted, got %d", len(res.deleted))
		}
		if len(res.added) != 1 || !res.added["y"] {
			t.Errorf("expected added y, got %v", res.added)
		}
	})

	t.Run("Deleted", func(t *testing.T) {
		a := map[string]bool{"x": true, "y": true}
		b := map[string]bool{"x": true}
		res := mapDiff(a, b)
		if len(res.added) != 0 {
			t.Errorf("expected no added, got %d", len(res.added))
		}
		if len(res.deleted) != 1 || !res.deleted["y"] {
			t.Errorf("expected deleted y, got %v", res.deleted)
		}
	})

	t.Run("Both", func(t *testing.T) {
		a := map[string]bool{"a": true}
		b := map[string]bool{"b": true}
		res := mapDiff(a, b)
		if len(res.deleted) != 1 || !res.deleted["a"] {
			t.Errorf("expected deleted a, got %v", res.deleted)
		}
		if len(res.added) != 1 || !res.added["b"] {
			t.Errorf("expected added b, got %v", res.added)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		res := mapDiff(nil, nil)
		if len(res.added) != 0 {
			t.Errorf("expected no added, got %d", len(res.added))
		}
		if len(res.deleted) != 0 {
			t.Errorf("expected no deleted, got %d", len(res.deleted))
		}
	})
}

func TestMapIntersection(t *testing.T) {
	t.Run("Overlapping", func(t *testing.T) {
		a := map[string]bool{"x": true, "y": true, "z": true}
		b := map[string]bool{"y": true, "z": true, "w": true}
		res := mapIntersection(a, b)
		if len(res) != 2 {
			t.Fatalf("expected 2 intersecting keys, got %d", len(res))
		}
		if !res["y"] {
			t.Error("expected y in intersection")
		}
		if !res["z"] {
			t.Error("expected z in intersection")
		}
	})

	t.Run("Disjoint", func(t *testing.T) {
		a := map[string]bool{"a": true}
		b := map[string]bool{"b": true}
		res := mapIntersection(a, b)
		if len(res) != 0 {
			t.Errorf("expected empty intersection, got %d", len(res))
		}
	})
}

func TestSplitKey(t *testing.T) {
	t.Run("FullKey", func(t *testing.T) {
		src, tgt, kw := splitKey("a|b|c")
		if src != "a" || tgt != "b" || kw != "c" {
			t.Errorf("expected a,b,c got %s,%s,%s", src, tgt, kw)
		}
	})

	t.Run("NoKeywords", func(t *testing.T) {
		src, tgt, kw := splitKey("a|b")
		if src != "a" || tgt != "b" || kw != "" {
			t.Errorf("expected a,b, got %s,%s,%s", src, tgt, kw)
		}
	})

	t.Run("Short", func(t *testing.T) {
		src, tgt, kw := splitKey("onlyone")
		if src != "" || tgt != "" || kw != "" {
			t.Errorf("expected all empty, got %s,%s,%s", src, tgt, kw)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		src, tgt, kw := splitKey("")
		if src != "" || tgt != "" || kw != "" {
			t.Errorf("expected all empty, got %s,%s,%s", src, tgt, kw)
		}
	})
}

func TestRelationKey(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		key := RelationKey("src", "tgt", "kw")
		if key != "src|tgt|kw" {
			t.Errorf("expected src|tgt|kw, got %s", key)
		}
	})

	t.Run("EmptyParts", func(t *testing.T) {
		key := RelationKey("", "", "")
		if key != "||" {
			t.Errorf("expected ||, got %s", key)
		}
	})
}
