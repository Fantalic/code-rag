package pipeline

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coderag/crud"
	"github.com/coderag/diff"
	"github.com/coderag/filter"
	"github.com/coderag/parser"
	"github.com/coderag/status"
)

// --- Zig pipeline -------------------------------------------------
//
// runZigPipeline is the Zig equivalent of runCSharpPipeline. Zig has
// no .sln/.csproj equivalent — the "project" is just a directory
// containing .zig files. We walk upward from the changed file looking
// for a build.zig (treated as the project root) and feed the
// directory to the analyser.

func runZigPipeline(filePath string, client *crud.Config, results []AnalyserResult) error {
	projectDir := findZigProjectDir(filePath)
	if projectDir == "" {
		return fmt.Errorf("no Zig project root found for %s", filePath)
	}

	if results == nil {
		var err error
		results, err = RunZigAnalyser(projectDir, "")
		if err != nil {
			return fmt.Errorf("Zig analyser failed: %w", err)
		}
	}

	if len(results) == 0 {
		return fmt.Errorf("Zig analyser produced no output for %s", projectDir)
	}

	return applyZigResults(filePath, client, results)
}

func runZigPipelineWithResults(filePath string, client *crud.Config, results []AnalyserResult) error {
	projectDir := findZigProjectDir(filePath)
	if projectDir == "" {
		return fmt.Errorf("no Zig project root found for %s", filePath)
	}

	if len(results) == 0 {
		return fmt.Errorf("no analyser results for %s", projectDir)
	}

	return applyZigResults(filePath, client, results)
}

func runZigPipelineModified(filePath string, client *crud.Config, results []AnalyserResult) error {
	log.Printf("Modified Zig file detected: %s, refreshing entities", filePath)

	if err := client.DeleteEntitiesByFilePath(filePath); err != nil {
		log.Printf("Warning: failed to delete old entities for %s: %v", filePath, err)
	}

	if len(results) == 0 {
		projectDir := findZigProjectDir(filePath)
		if projectDir == "" {
			return fmt.Errorf("no Zig project root found for %s", filePath)
		}
		var err error
		results, err = RunZigAnalyser(projectDir, "")
		if err != nil {
			return fmt.Errorf("Zig analyser failed: %w", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("Zig analyser produced no output for %s", projectDir)
		}
	}

	return runZigPipelineWithResults(filePath, client, results)
}

// applyZigResults pushes the analyser output into the LightRAG graph
// using the same entity/relation diff flow as the C# path. Used by
// the per-file watcher path (runZigPipelineModified does the
// DeleteEntitiesByFilePath pre-clear; this function then diffs the
// fresh analyser output against the remaining graph).
func applyZigResults(filePath string, client *crud.Config, results []AnalyserResult) error {
	_, _, _, _, _, _, err := applyZigResultsCounted(filePath, client, results)
	return err
}

// ApplyZigScanResults is the full-project scan variant used by
// `coderag scan`. The diff catches any entities missing from the
// new result set as ActionDelete, so no per-file pre-clear is needed.
func ApplyZigScanResults(client *crud.Config, projectDir string, results []AnalyserResult) error {
	_, _, _, _, _, _, err := applyZigResultsCounted(projectDir, client, results)
	return err
}

// applyZigResultsCounted is the shared body for both the per-file
// watcher path and the full-project scan path. Returns the entity
// and relation create/update/delete counts.
func applyZigResultsCounted(filePath string, client *crud.Config, results []AnalyserResult) (createdEntities, updatedEntities, deletedEntities, createdRels, updatedRels, deletedRels int, err error) {
	applyCSharpResultsMu.Lock()
	defer applyCSharpResultsMu.Unlock()

	log.Printf("APPLY ZIG RESULTS: processing %d results for %s", len(results), filePath)
	parserResults := make([]struct {
		Filename string
		Content  string
	}, len(results))
	for i, r := range results {
		parserResults[i] = struct {
			Filename string
			Content  string
		}{r.Filename, r.Content}
	}

	entities, relations := parser.ParseZigOutput(parserResults)

	log.Printf("Zig analyser extracted %d entities, %d relations from %s",
		len(entities), len(relations), filePath)

	currentEntities, getErr := client.GetAllEntities()
	if getErr != nil {
		currentEntities = []map[string]any{}
	}

	oldEntityMap := entitySliceToMap(currentEntities)
	newEntityMap := parserEntitiesToMap(entities)

	entityActions := diff.DiffEntities(oldEntityMap, newEntityMap)

	var entityBatch []crud.BatchEntityItem

	for _, action := range entityActions {
		switch action.Kind {
		case diff.ActionDelete:
			if _, delErr := client.DeleteEntity(action.EntityName); delErr != nil {
				log.Printf("ERROR deleting entity %q: %v", action.EntityName, delErr)
			} else {
				deletedEntities++
			}
		case diff.ActionCreate:
			data := filter.FilterEntityData(action.Data)
			entityBatch = append(entityBatch, crud.BatchEntityItem{
				EntityName: action.EntityName,
				EntityData: crud.EntityData{
					Description: filter.FilterDescription(getString(data, "description")),
					EntityType:  getString(data, "entity_type", "UNKNOWN"),
					SourceID:    getString(data, "source_id", filePath),
					FilePath:    getString(data, "file_path", filePath),
				},
			})
		case diff.ActionUpdate:
			data := filter.FilterEntityData(action.Data)
			if _, updErr := client.UpdateEntity(action.EntityName, data); updErr != nil {
				log.Printf("ERROR updating entity %q: %v", action.EntityName, updErr)
			} else {
				updatedEntities++
			}
		}
	}

	oldRelMap := loadCurrentRelations(client)
	newRelMap := parserRelationsToMap(relations)
	relActions := diff.DiffRelations(oldRelMap, newRelMap)

	var relBatch []crud.BatchRelationItem

	for _, action := range relActions {
		switch action.Kind {
		case diff.ActionDelete:
			if _, delErr := client.DeleteRelation(action.Source, action.Target); delErr != nil {
				log.Printf("ERROR deleting relation %q -> %q: %v", action.Source, action.Target, delErr)
			} else {
				deletedRels++
			}
		case diff.ActionCreate:
			key := dedupKey{src: action.Source, tgt: action.Target, kw: action.Keywords}
			if _, loaded := relationDedup.LoadOrStore(key, true); loaded {
				log.Printf("RELATION SKIP (dedup): %s -> %s kw=%q", action.Source, action.Target, action.Keywords)
				continue
			}

			data := filter.FilterRelationData(action.Data)
			relBatch = append(relBatch, crud.BatchRelationItem{
				SourceEntity: action.Source,
				TargetEntity: action.Target,
				RelationData: crud.RelationData{
					Description: filter.FilterDescription(getString(data, "description")),
					Keywords:    getString(data, "keywords"),
					Weight:      getFloat(data, "weight", 1.0),
					SourceID:    getString(data, "source_id", filePath),
					FilePath:    getString(data, "file_path", filePath),
				},
			})
		case diff.ActionUpdate:
			data := filter.FilterRelationData(action.Data)
			if _, updErr := client.UpdateRelation(action.Source, action.Target, data); updErr != nil {
				log.Printf("ERROR updating relation %q -> %q: %v", action.Source, action.Target, updErr)
			} else {
				updatedRels++
				log.Printf("RELATION UPDATE OK: %s -> %s", action.Source, action.Target)
			}
		}
	}

	if len(entityBatch) > 0 || len(relBatch) > 0 {
		result, batchErr := client.BatchImport(entityBatch, relBatch)
		if batchErr == nil {
			if data, ok := result["data"].(map[string]any); ok {
				if c, ok := data["entities_created"].(float64); ok {
					createdEntities += int(c)
				}
				if c, ok := data["relations_created"].(float64); ok {
					createdRels += int(c)
				}
			}
		} else {
			log.Printf("ERROR batch import: %v", batchErr)
			err = batchErr
			return
		}
	}

	log.Printf("Zig entities (local): created=%d, updated=%d, deleted=%d", createdEntities, updatedEntities, deletedEntities)
	log.Printf("Zig relations (local): created=%d, updated=%d, deleted=%d", createdRels, updatedRels, deletedRels)

	status.Default.Counts("crud", map[string]int{
		"files":             len(results),
		"entities_created":  createdEntities,
		"entities_updated":  updatedEntities,
		"entities_deleted":  deletedEntities,
		"relations_created": createdRels,
		"relations_updated": updatedRels,
		"relations_deleted": deletedRels,
	})
	status.Default.Detail("done", filePath,
		"Zig graph updated: %d entities (+%d/-%d/~%d), %d relations (+%d/-%d/~%d)",
		len(entities), createdEntities, deletedEntities, updatedEntities,
		len(relations), createdRels, deletedRels, updatedRels)

	return
}

// RunZigAnalyser shells out to zig_analyser.exe with the project dir.
// Output is read from the temp directory the analyser writes into.
// Honours ZIG_ANALYSER_PATH for non-canonical installs.
func RunZigAnalyser(projectDir string, outputDir string) ([]AnalyserResult, error) {
	analyserExe := os.Getenv("ZIG_ANALYSER_PATH")
	if analyserExe == "" {
		analyserExe = findZigAnalyser()
	}
	if analyserExe == "" {
		return nil, fmt.Errorf("zig_analyser not found; build it with 'cd coderag/cli/analysers/zig && go build -o zig_analyser.exe ./cmd/zig_analyser/' or set ZIG_ANALYSER_PATH")
	}

	if outputDir == "" {
		outputDir = filepath.Join(projectDir, ".coderag", "zig_analyser_tmp")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return nil, fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(outputDir)
	} else {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return nil, fmt.Errorf("create output dir: %w", err)
		}
		entries, _ := os.ReadDir(outputDir)
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
				continue
			}
			_ = os.Remove(filepath.Join(outputDir, entry.Name()))
		}
	}

	cmd := exec.Command(analyserExe, "scan", "--dir", projectDir, "--output", outputDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running Zig analyser on %s -> %s", projectDir, outputDir)
	status.Default.Stage("analyse", "running Zig analyser")
	if err := cmd.Run(); err != nil {
		status.Default.Error("analyse", "Zig analyser failed: %v", err)
		return nil, fmt.Errorf("run analyser: %w", err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("read output dir: %w", err)
	}

	var results []AnalyserResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		if entry.Name() == "_index.txt" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(outputDir, entry.Name()))
		if err != nil {
			continue
		}
		results = append(results, AnalyserResult{entry.Name(), string(content)})
	}

	log.Printf("Zig analyser produced %d output files", len(results))
	status.Default.Counts("analyse", map[string]int{"files": len(results)})
	return results, nil
}

// findZigAnalyser locates zig_analyser.exe by probing the same
// well-known locations as findCSharpAnalyser (env var, repo-relative,
// executable-relative).
func findZigAnalyser() string {
	find := func(c string) string {
		if c == "" {
			return ""
		}
		abs, err := filepath.Abs(c)
		if err != nil {
			return ""
		}
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
		return ""
	}

	names := func(dir string) []string {
		return []string{
			filepath.Join(dir, "zig_analyser.exe"),
			filepath.Join(dir, "zig_analyser"),
		}
	}

	for _, p := range []string{
		"coderag/cli/analysers/zig/zig_analyser.exe",
		"coderag/cli/analysers/zig/zig_analyser",
		"./coderag/cli/analysers/zig/zig_analyser.exe",
		"./coderag/cli/analysers/zig/zig_analyser",
		"../coderag/cli/analysers/zig/zig_analyser.exe",
		"../coderag/cli/analysers/zig/zig_analyser",
	} {
		if f := find(p); f != "" {
			return f
		}
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		for _, dir := range []string{
			filepath.Join(exeDir, "analysers", "zig"),
			filepath.Join(exeDir, "coderag", "cli", "analysers", "zig"),
		} {
			for _, c := range names(dir) {
				if f := find(c); f != "" {
					return f
				}
			}
		}
	}

	if repo := repoRoot(); repo != "" {
		for _, c := range names(filepath.Join(repo, "coderag", "cli", "analysers", "zig")) {
			if f := find(c); f != "" {
				return f
			}
		}
	}

	return ""
}

// findZigProjectDir walks up from a .zig file looking for a build.zig
// marker. Falls back to the file's containing directory when nothing
// is found (the analyser will still walk it).
func findZigProjectDir(filePath string) string {
	dir := filepath.Dir(filePath)
	for i := 0; i < 10; i++ {
		matches, _ := filepath.Glob(filepath.Join(dir, "build.zig"))
		if len(matches) > 0 {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// No build.zig — use the file's directory as the project root.
	return filepath.Dir(filePath)
}
