package parser

import (
	"bufio"
	"bytes"
	"path/filepath"
	"regexp"
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
	Source     string
	Target     string
	Description string
	Keywords    string
	Weight      float64
}

type ParseResult struct {
	Entities  []Entity
	Relations []Relation
}

var (
	methodPattern   = regexp.MustCompile(`METHOD:\s*([^\s->]+)\s*->\s*RETURNS\s*->\s*(.+)`)
	propertyPattern = regexp.MustCompile(`PROPERTY:\s*(\w+)\s*->\s*RETURNS\s*->\s*(.+)`)
	inheritsPattern = regexp.MustCompile(`INHERITS_FROM\s*->\s*(\w+)`)
	dependsPattern  = regexp.MustCompile(`DEPENDS_ON\s*->\s*(\w+)`)
	callsPattern    = regexp.MustCompile(`METHOD:\s*([^\s->]+)\s*->\s*CALLS\s*->\s*.+\s*\[LINK:\s*([^\]]+)\]`)
)

func cleanFilePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, " [LINK:"); idx >= 0 {
		raw = raw[:idx]
	}
	return strings.TrimSpace(raw)
}

type classAccum struct {
	name         string
	methods      []string
	properties   []string
	inheritsFrom string
	dependsOn    []string
	filePath     string
	sourceID     string
}

func flushClass(acc *classAccum, entities *[]Entity, relations *[]Relation) {
	if acc == nil || acc.name == "" {
		return
	}

	className := acc.name
	filePath := acc.filePath
	filename := acc.sourceID
	methods := acc.methods
	properties := acc.properties
	inheritsFrom := acc.inheritsFrom
	dependsOn := acc.dependsOn

	*entities = append(*entities, Entity{
		Name:        className,
		EntityType:  "CLASS",
		Description: className + " with " + itoa(len(methods)) + " method(s), " + itoa(len(properties)) + " properties",
		SourceID:    filename,
		FilePath:    filePath,
		Methods:     methods,
	})

	for _, method := range methods {
		*entities = append(*entities, Entity{
			Name:        method,
			EntityType:  "METHOD",
			Description: className + "." + method + "()",
			SourceID:    filename,
			FilePath:    filePath,
		})
		*relations = append(*relations, Relation{
			Source:      className,
			Target:      method,
			Description: className + "." + method + "()",
			Keywords:    "method, defines",
			Weight:      1.0,
		})
	}

	for _, prop := range properties {
		*entities = append(*entities, Entity{
			Name:        prop,
			EntityType:  "PROPERTY",
			Description: className + "." + prop,
			SourceID:    filename,
			FilePath:    filePath,
		})
		*relations = append(*relations, Relation{
			Source:      className,
			Target:      prop,
			Description: className + "." + prop,
			Keywords:    "property, has",
			Weight:      0.8,
		})
	}

	if inheritsFrom != "" {
		*relations = append(*relations, Relation{
			Source:      className,
			Target:      inheritsFrom,
			Description: className + " inherits from " + inheritsFrom,
			Keywords:    "inherits_from, extends",
			Weight:      1.0,
		})
	}

	for _, dep := range dependsOn {
		*relations = append(*relations, Relation{
			Source:      className,
			Target:      dep,
			Description: className + " implements " + dep,
			Keywords:    "depends_on, implements",
			Weight:      1.0,
		})
	}

	// Clear accumulator for next class
	acc.name = ""
	acc.methods = nil
	acc.properties = nil
	acc.inheritsFrom = ""
	acc.dependsOn = nil
}

func ParseCSharpFile(filename string, content string) (entities []Entity, relations []Relation) {
	entities = []Entity{}
	relations = []Relation{}

	// C# analyser output files use UTF-8 BOM (\ufeff). Strip it.
	content = strings.TrimLeft(content, "\ufeff")

	var filePath string
	var projectRefs []string
	inRefs := false
	hasClass := false

	var cur *classAccum

	startClass := func(name string) {
		flushClass(cur, &entities, &relations)
		cur = &classAccum{
			name:     name,
			filePath: filePath,
			sourceID: filename,
		}
		hasClass = true
	}

	scanner := bufio.NewScanner(bytes.NewReader([]byte(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "FILE:") {
			// Starting a new file section — flush pending class
			flushClass(cur, &entities, &relations)
			filePath = cleanFilePath(strings.TrimPrefix(line, "FILE:"))
			inRefs = false
		} else if strings.HasPrefix(line, "CLASS:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "CLASS:"))
			startClass(name)
			inRefs = false
		} else if cur == nil {
			continue
		} else if strings.HasPrefix(line, "METHOD:") {
			var methodName string
			if m := methodPattern.FindStringSubmatch(line); len(m) >= 2 {
				methodName = m[1]
				cur.methods = append(cur.methods, methodName)
			}
			if c := callsPattern.FindStringSubmatch(line); len(c) >= 2 && methodName != "" {
				callsTo := c[2]
				relations = append(relations, Relation{
					Source:      methodName,
					Target:      callsTo,
					Description: methodName + " calls method in " + callsTo,
					Keywords:    "calls, invokes",
					Weight:      0.9,
				})
			}
		} else if strings.HasPrefix(line, "PROPERTY:") {
			if p := propertyPattern.FindStringSubmatch(line); len(p) >= 2 {
				cur.properties = append(cur.properties, p[1])
			}
		} else if strings.HasPrefix(line, "INHERITS_FROM") {
			if m := inheritsPattern.FindStringSubmatch(line); len(m) >= 2 {
				cur.inheritsFrom = m[1]
			}
		} else if strings.HasPrefix(line, "DEPENDS_ON") {
			if m := dependsPattern.FindStringSubmatch(line); len(m) >= 2 {
				cur.dependsOn = append(cur.dependsOn, m[1])
			}
		} else if strings.HasPrefix(line, "PROJECT REFERENCES:") {
			inRefs = true
		} else if inRefs && strings.HasPrefix(line, "-") {
			ref := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if ref != "" {
				projectRefs = append(projectRefs, ref)
			}
		}
	}

	// Flush the last class
	flushClass(cur, &entities, &relations)

	if !hasClass {
		// Still return project-ref relations if no class entities
		for _, ref := range projectRefs {
			relations = append(relations, Relation{
				Source:      filename,
				Target:      ref,
				Description: filename + " references project " + ref,
				Keywords:    "project_reference, dependency",
				Weight:      1.0,
			})
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
			key := rel.Source + "|" + rel.Target
			if !seenRelations[key] {
				seenRelations[key] = true
				allRelations = append(allRelations, rel)
			}
		}
	}

	// Build map of canonical filePath per class
	// A CLASS entity is "canonical" when its filePath basename matches the class name
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
				// For CLASS: prefer where filePath basename matches class name
				for _, c := range candidates {
					base := strings.TrimSuffix(filepath.Base(c.FilePath), ".cs")
					if base == c.Name {
						chosen = c
						break
					}
				}
			} else {
				// For METHOD/PROPERTY: prefer filePath matching canonical parent class
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