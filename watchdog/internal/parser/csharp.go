package parser

import (
	"bufio"
	"bytes"
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

func ParseCSharpFile(filename string, content string) (entities []Entity, relations []Relation) {
	entities = []Entity{}
	relations = []Relation{}

	var filePath, className string
	var methods []string
	var properties []string
	var projectRefs []string
	var inheritsFrom string
	var dependsOn []string
	inRefs := false

	scanner := bufio.NewScanner(bytes.NewReader([]byte(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "FILE:") {
			filePath = strings.TrimPrefix(line, "FILE:")
			filePath = strings.TrimSpace(filePath)
		} else if strings.HasPrefix(line, "CLASS:") {
			className = strings.TrimPrefix(line, "CLASS:")
			className = strings.TrimSpace(className)
			methods = []string{}
			properties = []string{}
			projectRefs = []string{}
			inheritsFrom = ""
			dependsOn = []string{}
			inRefs = false
		} else if strings.HasPrefix(line, "METHOD:") {
			var methodName string
			if m := methodPattern.FindStringSubmatch(line); len(m) >= 2 {
				methodName = m[1]
				methods = append(methods, methodName)
			}
			if c := callsPattern.FindStringSubmatch(line); len(c) >= 2 && methodName != "" {
				callsTo := c[2]
				relations = append(relations, Relation{
					Source:     methodName,
					Target:     callsTo,
					Description: methodName + " calls method in " + callsTo,
					Keywords:    "calls, invokes",
					Weight:      0.9,
				})
			}
		} else if strings.HasPrefix(line, "PROPERTY:") {
			if p := propertyPattern.FindStringSubmatch(line); len(p) >= 2 {
				properties = append(properties, p[1])
			}
		} else if strings.HasPrefix(line, "INHERITS_FROM") {
			if m := inheritsPattern.FindStringSubmatch(line); len(m) >= 2 {
				inheritsFrom = m[1]
			}
		} else if strings.HasPrefix(line, "DEPENDS_ON") {
			if m := dependsPattern.FindStringSubmatch(line); len(m) >= 2 {
				dependsOn = append(dependsOn, m[1])
			}
		} else if strings.HasPrefix(line, "PROJECT REFERENCES:") {
			inRefs = true
		} else if inRefs && strings.HasPrefix(line, "-") {
			ref := strings.TrimPrefix(line, "-")
			ref = strings.TrimSpace(ref)
			if ref != "" {
				projectRefs = append(projectRefs, ref)
			}
		}
	}

	if className == "" {
		return nil, nil
	}

	entity := Entity{
		Name:        className,
		EntityType:  "CLASS",
		Description: className + " with " + itoa(len(methods)) + " method(s), " + itoa(len(properties)) + " properties",
		SourceID:    filename,
		FilePath:    filePath,
		Methods:     methods,
	}
	entities = append(entities, entity)

	for _, method := range methods {
		methodEntity := Entity{
			Name:        method,
			EntityType:  "METHOD",
			Description: className + "." + method + "()",
			SourceID:    filename,
			FilePath:    filePath,
		}
		entities = append(entities, methodEntity)

		relations = append(relations, Relation{
			Source:     className,
			Target:     method,
			Description: className + "." + method + "()",
			Keywords:    "method, defines",
			Weight:      1.0,
		})
	}

	for _, prop := range properties {
		propEntity := Entity{
			Name:        prop,
			EntityType:  "PROPERTY",
			Description: className + "." + prop,
			SourceID:    filename,
			FilePath:    filePath,
		}
		entities = append(entities, propEntity)

		relations = append(relations, Relation{
			Source:     className,
			Target:     prop,
			Description: className + "." + prop,
			Keywords:    "property, has",
			Weight:      0.8,
		})
	}

	if inheritsFrom != "" {
		relations = append(relations, Relation{
			Source:     className,
			Target:     inheritsFrom,
			Description: className + " inherits from " + inheritsFrom,
			Keywords:    "inherits_from, extends",
			Weight:      1.0,
		})
	}

	for _, dep := range dependsOn {
		relations = append(relations, Relation{
			Source:     className,
			Target:     dep,
			Description: className + " implements " + dep,
			Keywords:    "depends_on, implements",
			Weight:      1.0,
		})
	}

	for _, ref := range projectRefs {
		relations = append(relations, Relation{
			Source:     className,
			Target:     ref,
			Description: className + " references project " + ref,
			Keywords:    "project_reference, dependency",
			Weight:      1.0,
		})
	}

	return entities, relations
}

func ParseCSharpOutput(results []struct {
	Filename string
	Content  string
}) (allEntities []Entity, allRelations []Relation) {
	seenEntities := make(map[string]bool)
	seenRelations := make(map[string]bool)

	for _, r := range results {
		entities, relations := ParseCSharpFile(r.Filename, r.Content)
		for _, e := range entities {
			if !seenEntities[e.Name] {
				seenEntities[e.Name] = true
				allEntities = append(allEntities, e)
			}
		}
		for _, rel := range relations {
			key := rel.Source + "|" + rel.Target
			if !seenRelations[key] {
				seenRelations[key] = true
				allRelations = append(allRelations, rel)
			}
		}
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