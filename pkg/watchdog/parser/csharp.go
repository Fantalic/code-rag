package parser

import (
	"bufio"
	"bytes"
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

// format: ENTITY_TYPE|Name|FilePath|LineNo|Description
func parseEntityLine(line string) (Entity, bool) {
	parts := strings.SplitN(line, "|", 5)
	if len(parts) < 5 {
		return Entity{}, false
	}
	entityType := parts[0]
	name := parts[1]
	filePath := parts[2]
	desc := parts[4]

	// Basic validation
	if name == "" || entityType == "" {
		return Entity{}, false
	}

	return Entity{
		Name:        name,
		EntityType:  entityType,
		Description: desc,
		FilePath:    filePath,
	}, true
}

// format: CALL|CallerFile|CallerMethod|CalleeName|CalleeClass|LineNo
func parseCallLine(line string) (Relation, bool) {
	parts := strings.SplitN(line, "|", 6)
	if len(parts) < 6 {
		return Relation{}, false
	}
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
	classMethods := make(map[string][]string)
	seenEntities := make(map[string]bool)

	scanner := bufio.NewScanner(bytes.NewReader([]byte(content)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Entity lines: CLASS|, INTERFACE|, STRUCT|, ENUM|, METHOD|, PROPERTY|
		if trimmed[0] >= 'A' && trimmed[0] <= 'Z' {
			colonIdx := strings.Index(trimmed, "|")
			if colonIdx > 0 {
				prefix := trimmed[:colonIdx]
				switch prefix {
				case "CLASS", "INTERFACE", "STRUCT", "ENUM", "METHOD", "PROPERTY":
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
						relations = append(relations, rel)
					}
					continue
			case "RELATION":
				rel, ok := parseRelationLine(trimmed)
				if ok {
					rel.SourceID = filename
					relations = append(relations, rel)
					// Track methods per class for the CLASS entity's Methods field
						if rel.Keywords == "method, defines" {
							classMethods[rel.Source] = append(classMethods[rel.Source], rel.Target)
						}
					}
					continue
				}
			}
		}
	}

	// Populate Methods field on CLASS entities
	for i, e := range entities {
		if e.EntityType == "CLASS" {
			if methods, ok := classMethods[e.Name]; ok {
				entities[i].Methods = methods
				// Update description with method/property count
				propCount := 0
				for _, rel := range relations {
					if rel.Source == e.Name && rel.Keywords == "property, has" {
						propCount++
					}
				}
				entities[i].Description = e.Name + " with " + itoa(len(methods)) + " method(s), " + itoa(propCount) + " properties"
			}
		}
	}

	return entities, relations
}

func ParseCSharpOutput(results []struct {
	Filename string
	Content  string
}) (allEntities []Entity, allRelations []Relation) {
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

	// Build map of canonical filePath per class
	canonicalClassFile := make(map[string]string)
	for name, candidates := range entityMap {
		if len(candidates) == 0 || candidates[0].EntityType != "CLASS" {
			continue
		}
		canonical := candidates[0].FilePath
		for _, c := range candidates {
			base := strings.TrimSuffix(filepath.Base(c.FilePath), ".cs")
			if base == c.Name {
				canonical = c.FilePath
				break
			}
		}
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
			if candidates[0].EntityType == "CLASS" {
				for _, c := range candidates {
					base := strings.TrimSuffix(filepath.Base(c.FilePath), ".cs")
					if base == c.Name {
						chosen = c
						break
					}
				}
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

