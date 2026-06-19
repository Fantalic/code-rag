package diff

type ActionKind string

const (
	ActionCreate ActionKind = "CREATE"
	ActionUpdate ActionKind = "UPDATE"
	ActionDelete ActionKind = "DELETE"
)

type EntityAction struct {
	Kind       ActionKind
	EntityName string
	Data       map[string]any
}

func NewEntityAction(kind ActionKind, entityName string, data map[string]any) *EntityAction {
	if data == nil {
		data = map[string]any{}
	}
	return &EntityAction{Kind: kind, EntityName: entityName, Data: data}
}

type RelationAction struct {
	Kind   ActionKind
	Source string
	Target string
	Data   map[string]any
}

func NewRelationAction(kind ActionKind, source, target string, data map[string]any) *RelationAction {
	if data == nil {
		data = map[string]any{}
	}
	return &RelationAction{Kind: kind, Source: source, Target: target, Data: data}
}

var ignoredKeys = map[string]bool{
	"created_at": true,
	"source_id":  true,
	"entity_id":  true,
	"src_id":     true,
	"tgt_id":     true,
}

func dataChanged(a, b map[string]any) bool {
	filteredA := filterKeys(a)
	filteredB := filterKeys(b)
	return !equalMaps(filteredA, filteredB)
}

func filterKeys(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		if !ignoredKeys[k] {
			result[k] = v
		}
	}
	return result
}

func equalMaps(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || !equalValues(v, bv) {
			return false
		}
	}
	return true
}

func equalValues(a, b any) bool {
	am, ok := a.(map[string]any)
	bm, ok2 := b.(map[string]any)
	if ok && ok2 {
		return equalMaps(am, bm)
	}
	ab, ok := a.([]any)
	bb, ok2 := b.([]any)
	if ok && ok2 {
		if len(ab) != len(bb) {
			return false
		}
		for i := range ab {
			if !equalValues(ab[i], bb[i]) {
				return false
			}
		}
		return true
	}
	return a == b
}

func DiffEntities(oldEntities, newEntities map[string]map[string]any) []*EntityAction {
	var actions []*EntityAction

	oldNames := keySet(oldEntities)
	newNames := keySet(newEntities)

	for name := range mapDiff(oldNames, newNames).deleted {
		actions = append(actions, NewEntityAction(ActionDelete, name, nil))
	}

	for name := range mapDiff(oldNames, newNames).added {
		actions = append(actions, NewEntityAction(ActionCreate, name, newEntities[name]))
	}

	for name := range mapIntersection(oldNames, newNames) {
		if dataChanged(oldEntities[name], newEntities[name]) {
			actions = append(actions, NewEntityAction(ActionUpdate, name, newEntities[name]))
		}
	}

	return actions
}

func DiffRelations(oldRelations, newRelations map[string]map[string]any) []*RelationAction {
	var actions []*RelationAction

	oldKeys := keySet(oldRelations)
	newKeys := keySet(newRelations)

	for key := range mapDiff(oldKeys, newKeys).deleted {
		src, tgt := splitKey(key)
		actions = append(actions, NewRelationAction(ActionDelete, src, tgt, nil))
	}

	for key := range mapDiff(oldKeys, newKeys).added {
		src, tgt := splitKey(key)
		actions = append(actions, NewRelationAction(ActionCreate, src, tgt, newRelations[key]))
	}

	for key := range mapIntersection(oldKeys, newKeys) {
		src, tgt := splitKey(key)
		if dataChanged(oldRelations[key], newRelations[key]) {
			actions = append(actions, NewRelationAction(ActionUpdate, src, tgt, newRelations[key]))
		}
	}

	return actions
}

type diffResult struct {
	added   map[string]bool
	deleted map[string]bool
}

func mapDiff(a, b map[string]bool) diffResult {
	res := diffResult{
		added:   make(map[string]bool),
		deleted: make(map[string]bool),
	}
	for k := range a {
		if !b[k] {
			res.deleted[k] = true
		}
	}
	for k := range b {
		if !a[k] {
			res.added[k] = true
		}
	}
	return res
}

func mapIntersection(a, b map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for k := range a {
		if b[k] {
			result[k] = true
		}
	}
	return result
}

func keySet(m map[string]map[string]any) map[string]bool {
	s := make(map[string]bool)
	for k := range m {
		s[k] = true
	}
	return s
}

func splitKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			return key[:i], key[i+1:]
		}
	}
	return "", ""
}

func makeKey(src, tgt string) string {
	return src + "|" + tgt
}

func RelationKey(src, tgt string) string {
	return makeKey(src, tgt)
}