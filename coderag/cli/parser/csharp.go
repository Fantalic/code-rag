package parser

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Entity struct {
	Name        string
	EntityType  string
	Description string
	SourceID    string
	FilePath    string
	StartLine   int
	EndLine     int
	Methods     []string
}

type Relation struct {
	Source      string
	Target      string
	Description string
	Keywords    string
	Weight      float64
	SourceID    string
	FilePath    string
}

// DuplicatesConfig stores human-overridden canonical-file preferences for
// ambiguous class definitions (e.g. partial classes / projects that emit
// the same logical class multiple times). Keys are class names; values are
// absolute file paths whose match wins the canonical-file selection step.
type DuplicatesConfig map[string]string

// LoadDuplicatesConfig reads `<dir>/.coderag/duplicates.json`, falling back
// to `<home>/.coderag/duplicates.json` so per-project and global
// overrides can coexist. Returns an empty config silently on missing
// read — the caller's defaults stay in effect when no override file
// exists.
func LoadDuplicatesConfig(dir string) DuplicatesConfig {
	cfg := DuplicatesConfig{}
	for _, root := range duplicatesSearchRoots(dir) {
		path := filepath.Join(root, ".coderag", "duplicates.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			// A malformed override file is a developer mistake; surface it via
			// stderr so it's visible but never fatal.
			fmt.Fprintf(os.Stderr, "warning: parse %s: %v\n", path, err)
			return DuplicatesConfig{}
		}
		return cfg
	}
	return cfg
}

func duplicatesSearchRoots(dir string) []string {
	var roots []string
	if dir != "" {
		roots = append(roots, dir)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots, home)
	}
	return roots
}
//   - Legacy (v1): TYPE|Name|FilePath|LineNo|Description           (5 cols)
//   - Current (v2): TYPE|Name|FilePath|StartLine|EndLine|Description (6 cols)
// The output format emits a `# FORMAT: N` header so callers can be strict,
// but we keep parsing tolerant because scans can mix new + old results
// during a migration window.
func parseEntityLine(line string) (Entity, bool) {
	parts := strings.SplitN(line, "|", 6)
	if len(parts) < 5 {
		return Entity{}, false
	}
	entityType := parts[0]
	name := parts[1]
	filePath := parts[2]
	desc := ""
	start, end := 0, 0

	switch len(parts) {
	case 6:
		// v2: ...|StartLine|EndLine|Description
		start, _ = strconv.Atoi(strings.TrimSpace(parts[3]))
		end, _ = strconv.Atoi(strings.TrimSpace(parts[4]))
		desc = parts[5]
	default:
		// v1: ...|LineNo|Description
		start, _ = strconv.Atoi(strings.TrimSpace(parts[3]))
		end = start
		desc = parts[4]
	}

	if name == "" || entityType == "" {
		return Entity{}, false
	}

	return Entity{
		Name:        name,
		EntityType:  entityType,
		Description: desc,
		FilePath:    filePath,
		StartLine:   start,
		EndLine:     end,
	}, true
}

// format: CALL|CallerFile|CallerMethod|CalleeName|CalleeClass|LineNo
func parseCallLine(line string) (Relation, bool) {
	parts := strings.SplitN(line, "|", 6)
	if len(parts) < 6 {
		return Relation{}, false
	}
	callerFile := parts[1]
	callerMethod := parts[2]
	calleeName := parts[3]

	if callerMethod == "" || calleeName == "" {
		return Relation{}, false
	}

	return Relation{
		Source:      callerMethod,
		Target:      calleeName,
		Description: callerMethod + " calls " + calleeName,
		Keywords:    "calls",
		Weight:      0.5,
		FilePath:    callerFile,
	}, true
}

// format: RELATION|Source|Target|Description|Keywords|FilePath|Weight
func parseRelationLine(line string) (Relation, bool) {
	parts := strings.SplitN(line, "|", 7)
	if len(parts) < 7 {
		return Relation{}, false
	}

	source := parts[1]
	target := parts[2]
	desc := parts[3]
	keywords := parts[4]
	filePath := parts[5]
	weight, err := strconv.ParseFloat(parts[6], 64)
	if err != nil {
		weight = 1.0
	}

	if source == "" || target == "" {
		return Relation{}, false
	}

	return Relation{
		Source:      source,
		Target:      target,
		Description: desc,
		Keywords:    keywords,
		Weight:      weight,
		FilePath:    filePath,
	}, true
}

func ParseCSharpFile(filename string, content string) (entities []Entity, relations []Relation) {
	content = strings.TrimLeft(content, "\ufeff")

	// Track classes to populate Methods field
	classMembers := make(map[string][]string)
	seenEntities := make(map[string]bool)

	scanner := bufio.NewScanner(bytes.NewReader([]byte(content)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// COVERAGE|<project-key>|<json-or-status> is consumed by the indexer,
		// not by this parser — surface it as a synthetic entity carrying the
		// full diagnostic for downstream debugging tools.
		if strings.HasPrefix(trimmed, "COVERAGE|") {
			parts := strings.SplitN(trimmed, "|", 3)
			if len(parts) == 3 {
				entities = append(entities, Entity{
					Name:        "COVERAGE:" + parts[1],
					EntityType:  "COVERAGE",
					Description: parts[2],
					SourceID:    filename,
				})
			}
			continue
		}

		// CHANGELOG|<file>|<json> surfaces as a synthetic entity so the
		// MCP what_changed tool can filter by timestamp without ad-hoc
		// parsing per series.
		if strings.HasPrefix(trimmed, "CHANGELOG|") {
			parts := strings.SplitN(trimmed, "|", 3)
			if len(parts) == 3 {
				entities = append(entities, Entity{
					Name:        "CHANGELOG:" + parts[1],
					EntityType:  "CHANGELOG",
					Description: parts[2],
					SourceID:    filename,
				})
			}
			continue
		}

		// Entity lines: CLASS|, INTERFACE|, STRUCT|, ENUM|, METHOD|, PROPERTY|, FIELD|, CONSTRUCTOR|, ENUM_MEMBER|, RECORD|, LOCAL_FUNCTION|
		if len(trimmed) > 0 && trimmed[0] >= 'A' && trimmed[0] <= 'Z' {
			colonIdx := strings.Index(trimmed, "|")
			if colonIdx > 0 {
				prefix := trimmed[:colonIdx]
				switch prefix {
				case "CLASS", "INTERFACE", "STRUCT", "ENUM", "RECORD",
					"METHOD", "PROPERTY", "FIELD", "CONSTRUCTOR", "ENUM_MEMBER",
					"LOCAL_FUNCTION", "EXPORTS":
					ent, ok := parseEntityLine(trimmed)
					if ok {
						ent.SourceID = filename
						// Dedup by name+type
						key := ent.EntityType + ":" + ent.Name
						if !seenEntities[key] {
							seenEntities[key] = true
							entities = append(entities, ent)
						}
					}
					continue
				case "CALL":
					rel, ok := parseCallLine(trimmed)
					if ok {
						rel.SourceID = filename
						relations = append(relations, rel)
					}
					continue
				case "RELATION":
					rel, ok := parseRelationLine(trimmed)
					if ok {
						rel.SourceID = filename
						relations = append(relations, rel)
						// Track members per parent class for the aggregate description.
						if rel.Keywords == "method, defines" || rel.Keywords == "property, has" {
							classMembers[rel.Source] = append(classMembers[rel.Source], rel.Target)
						}
					}
					continue
				}
			}
		}
	}

	// Populate Methods/Description on aggregate members (classes, structs, etc.).
	for i, e := range entities {
		if e.EntityType != "CLASS" && e.EntityType != "INTERFACE" && e.EntityType != "STRUCT" && e.EntityType != "RECORD" {
			continue
		}
		// Methods field carries only callables (matches legacy semantics).
		var methods []string
		propCount := 0
		methodCount := 0
		for _, rel := range relations {
			if rel.Source != e.Name {
				continue
			}
			switch rel.Keywords {
			case "method, defines":
				methods = append(methods, rel.Target)
				methodCount++
			case "property, has":
				propCount++
			}
		}
		if len(methods) > 0 || propCount > 0 {
			entities[i].Methods = methods
			entities[i].Description = e.Name + " with " + itoa(methodCount) + " method(s), " + itoa(propCount) + " properties"
		}
	}

	return entities, relations
}

func ParseCSharpOutput(results []struct {
	Filename string
	Content  string
}) (allEntities []Entity, allRelations []Relation) {
	return ParseCSharpOutputWithConfig(results, nil)
}

// ParseCSharpOutputWithConfig is the merge step used by the pipeline. It
// collapses duplicates of the same class — by default picking the file whose
// base name matches the class — and lets a DuplicatesConfig override that
// pick when a project keeps partials of the same class in several files.
func ParseCSharpOutputWithConfig(results []struct {
	Filename string
	Content  string
}, override DuplicatesConfig,
) (allEntities []Entity, allRelations []Relation) {
	entityMap := make(map[string][]Entity)
	seenRelations := make(map[string]bool)

	for _, r := range results {
		entities, relations := ParseCSharpFile(r.Filename, r.Content)
		for _, e := range entities {
			entityMap[e.Name] = append(entityMap[e.Name], e)
		}
		for _, rel := range relations {
			key := rel.Source + "|" + rel.Target + "|" + rel.Keywords
			if !seenRelations[key] {
				seenRelations[key] = true
				allRelations = append(allRelations, rel)
			}
		}
	}

	// Build map of canonical filePath per class. Operator overrides (loaded
	// from .coderag/duplicates.json) win when they match a candidate.
	canonicalClassFile := make(map[string]string)
	for name, candidates := range entityMap {
		if len(candidates) == 0 || candidates[0].EntityType != "CLASS" {
			continue
		}
		canonical := candidates[0].FilePath
		if override != nil {
			if want, ok := override[name]; ok && want != "" {
				for _, c := range candidates {
					if c.FilePath == want {
						canonical = c.FilePath
						goto decided
					}
				}
			}
		}
		// Default to "file base name == class name" (one file per class).
		// Fall through to keep `canonical = candidates[0].FilePath` if no
		// candidate matches the heuristic, matching the legacy behaviour.
		for _, c := range candidates {
			base := strings.TrimSuffix(filepath.Base(c.FilePath), ".cs")
			if base == c.Name {
				canonical = c.FilePath
				break
			}
		}
	decided:
		canonicalClassFile[name] = canonical
	}

	// Build parent class map from relations
	parentClass := make(map[string]string)
	for _, rel := range allRelations {
		if rel.Keywords == "method, defines" || rel.Keywords == "property, has" {
			parentClass[rel.Target] = rel.Source
		}
	}

	// Pick best entity per name
	for name, candidates := range entityMap {
		if len(candidates) == 0 {
			continue
		}
		chosen := candidates[0]
		if len(candidates) > 1 {
			// Honour the operator override (or the legacy heuristic result
			// captured in canonicalClassFile) before falling back to
			// "first candidate" — otherwise override selections are silently
			// lost when no candidate's basename matches the class name.
			if candidates[0].EntityType == "CLASS" {
				if fp, ok := canonicalClassFile[name]; ok && fp != "" {
					for _, c := range candidates {
						if c.FilePath == fp {
							chosen = c
							goto classChosen
						}
					}
				}
				// Fallback: legacy base==name heuristic for projects that
				// do not ship a duplicates.json.
				for _, c := range candidates {
					base := strings.TrimSuffix(filepath.Base(c.FilePath), ".cs")
					if base == c.Name {
						chosen = c
						break
					}
				}
			classChosen:
			} else {
				if parent, ok := parentClass[name]; ok {
					if fp, ok := canonicalClassFile[parent]; ok && fp != "" {
						for _, c := range candidates {
							if c.FilePath == fp {
								chosen = c
								break
							}
						}
					}
				}
			}
		}
		allEntities = append(allEntities, chosen)
	}

	return allEntities, allRelations
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b strings.Builder
	for i > 0 {
		b.WriteByte(byte('0' + i%10))
		i /= 10
	}
	return reverse(b.String())
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

