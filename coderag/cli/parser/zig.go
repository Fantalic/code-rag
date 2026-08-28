package parser

// Zig analyser output parser. Mirrors cli/parser/csharp.go so the
// pipeline can ingest both languages through the same machinery.
//
// Each `.txt` sidecar produced by zig_analyser contains:
//   # FORMAT: 2                          (header)
//   # EXPORTS: Name1, Name2              (header)
//   # MTIME: <unix>                      (optional header)
//   # IMPORTS: Name1, Name2              (header)
//   FUNCTION|Name|FilePath|Start|End|Description
//   STRUCT|Name|...|Description
//   RELATION|Src|Tgt|Description|Keywords|FilePath|Weight
//   CALL|CallerFile|CallerMethod|CalleeName|CalleeOwner|LineNo
//
// Synthetic sidecars:
//   _coverage.txt   -> COVERAGE|<key>|{json}
//   _changelog.txt  -> CHANGELOG|<file>|{json}
//
// Format 2 added an end-line column to entity rows; the parser is
// tolerant of legacy 5-column rows during a migration window.

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"
)

// ZigFileResult is the parsed payload of one .txt sidecar. Used by the
// test suite and by future tools that want per-file inspection without
// going through ParseZigOutput's merge step.
type ZigFileResult struct {
	Entities  []Entity
	Relations []Relation
}

// ParseZigFile is the per-file equivalent of ParseCSharpFile: it walks
// the lines once, dispatching to the entity/relation/import/call/
// sidecar handlers.
func ParseZigFile(filename string, content string) (entities []Entity, relations []Relation) {
	content = strings.TrimLeft(content, "\ufeff")

	entitySeen := make(map[string]bool)
	relationSeen := make(map[string]bool)

	scanner := bufio.NewScanner(bytes.NewReader([]byte(content)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Sidecars — same shape as the C# analyser so downstream tools
		// (index_coverage, what_changed) read both uniformly.
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

		if len(trimmed) == 0 || trimmed[0] < 'A' || trimmed[0] > 'Z' {
			continue
		}
		prefix := trimmed
		if idx := strings.Index(trimmed, "|"); idx > 0 {
			prefix = trimmed[:idx]
		}
		switch prefix {
		case "FUNCTION", "STRUCT", "ENUM", "UNION", "OPAQUE",
			"ERROR_SET", "CONST", "VAR", "TEST", "ENUM_MEMBER",
			"EXPORTS", "IMPORT", "USING":
			ent, ok := parseEntityLine(trimmed)
			if ok {
				ent.SourceID = filename
				key := ent.EntityType + ":" + ent.Name
				if !entitySeen[key] {
					entitySeen[key] = true
					entities = append(entities, ent)
				}
			}
			continue
		case "RELATION":
			rel, ok := parseRelationLine(trimmed)
			if ok {
				rel.SourceID = filename
				key := rel.Source + "|" + rel.Target + "|" + rel.Keywords
				if !relationSeen[key] {
					relationSeen[key] = true
					relations = append(relations, rel)
				}
			}
			continue
		case "CALL":
			rel, ok := parseZigCallLine(trimmed)
			if ok {
				rel.SourceID = filename
				key := rel.Source + "|" + rel.Target + "|" + rel.Keywords + "|" + rel.FilePath
				if !relationSeen[key] {
					relationSeen[key] = true
					relations = append(relations, rel)
				}
			}
			continue
		}
	}
	return entities, relations
}

// parseZigCallLine decodes a CALL row from the Zig analyser. Schema:
// CALL|CallerFile|CallerMethod|CalleeName|CalleeOwner|LineNo
func parseZigCallLine(line string) (Relation, bool) {
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

// ParseZigOutput is the merge step used by the pipeline: it
// deduplicates by (name) for entities and (source|target|keywords) for
// relations, so the same constant declared in two files collapses to
// one entity. The merge picks the first occurrence deterministically.
func ParseZigOutput(results []struct {
	Filename string
	Content  string
}) (allEntities []Entity, allRelations []Relation) {
	entityMap := make(map[string][]Entity)
	seenRelations := make(map[string]bool)

	for _, r := range results {
		entities, relations := ParseZigFile(r.Filename, r.Content)
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

// Pick the first candidate per name; matches the C# analyser's
	// merge strategy (single declaration wins) without inventing a
	// duplicates.json override path — Zig has no partial-class concept.
	for _, candidates := range entityMap {
		if len(candidates) == 0 {
			continue
		}
		chosen := candidates[0]
		// Prefer the candidate whose basename matches the entity name —
		// catches the common case of one type per file.
		for _, c := range candidates {
			base := strings.TrimSuffix(filepath.Base(c.FilePath), ".zig")
			if base == c.Name {
				chosen = c
				break
			}
		}
		allEntities = append(allEntities, chosen)
	}

	return allEntities, allRelations
}
