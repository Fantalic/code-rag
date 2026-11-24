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

// runPythonPipeline runs the Python analyser on a project and applies
// the results to the LightRAG graph. Mirrors runZigPipeline for the
// Python language path.
func runPythonPipeline(filePath string, client *crud.Config, results []AnalyserResult) error {
	projectDir := findPythonProjectDir(filePath)
	if projectDir == "" {
		return fmt.Errorf("no Python project root found for %s", filePath)
	}

	if results == nil {
		var err error
		results, err = RunPythonAnalyser(projectDir, "")
		if err != nil {
			return fmt.Errorf("Python analyser failed: %w", err)
		}
	}

	if len(results) == 0 {
		return fmt.Errorf("Python analyser produced no output for %s", projectDir)
	}

	return applyPythonResults(filePath, client, results)
}

// runPythonPipelineWithResults applies pre-collected Python analyser
// output to the graph. Used when the caller already has results.
func runPythonPipelineWithResults(filePath string, client *crud.Config, results []AnalyserResult) error {
	projectDir := findPythonProjectDir(filePath)
	if projectDir == "" {
		return fmt.Errorf("no Python project root found for %s", filePath)
	}

	if len(results) == 0 {
		return fmt.Errorf("no analyser results for %s", projectDir)
	}

	return applyPythonResults(filePath, client, results)
}

// runPythonPipelineModified handles a modified .py file: it first
// deletes the old entities for that file, then re-runs the analyser
// (or uses supplied results) and applies the fresh output.
func runPythonPipelineModified(filePath string, client *crud.Config, results []AnalyserResult) error {
	log.Printf("Modified Python file detected: %s, refreshing entities", filePath)

	if err := client.DeleteEntitiesByFilePath(filePath); err != nil {
		log.Printf("Warning: failed to delete old entities for %s: %v", filePath, err)
	}

	if len(results) == 0 {
		projectDir := findPythonProjectDir(filePath)
		if projectDir == "" {
			return fmt.Errorf("no Python project root found for %s", filePath)
		}
		var err error
		results, err = RunPythonAnalyser(projectDir, "")
		if err != nil {
			return fmt.Errorf("Python analyser failed: %w", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("Python analyser produced no output for %s", projectDir)
		}
	}

	return runPythonPipelineWithResults(filePath, client, results)
}

// applyPythonResults is the thin wrapper used by the per-file watcher.
// Mirrors applyZigResults; calls parsePythonOutput and delegates to
// applyPythonResultsCounted.
func applyPythonResults(filePath string, client *crud.Config, results []AnalyserResult) error {
	_, _, _, _, _, _, err := applyPythonResultsCounted(filePath, client, results)
	return err
}

// ApplyPythonScanResults is the full-project scan variant for
// `coderag scan`. Mirrors ApplyZigScanResults.
func ApplyPythonScanResults(client *crud.Config, projectDir string, results []AnalyserResult) error {
	_, _, _, _, _, _, err := applyPythonResultsCounted(projectDir, client, results)
	return err
}

// applyPythonResultsCounted is the shared body for both the per-file
// watcher and the full-project scan paths. Mirrors applyZigResultsCounted
// but uses parser.ParsePythonOutput. Returns the entity and relation
// create/update/delete counts.
func applyPythonResultsCounted(filePath string, client *crud.Config, results []AnalyserResult) (createdEntities, updatedEntities, deletedEntities, createdRels, updatedRels, deletedRels int, err error) {
	applyCSharpResultsMu.Lock()
	defer applyCSharpResultsMu.Unlock()

	log.Printf("APPLY PYTHON RESULTS: processing %d results for %s", len(results), filePath)
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

	entities, relations := parser.ParsePythonOutput(parserResults)

	log.Printf("Python analyser extracted %d entities, %d relations from %s",
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

	log.Printf("Python entities (local): created=%d, updated=%d, deleted=%d", createdEntities, updatedEntities, deletedEntities)
	log.Printf("Python relations (local): created=%d, updated=%d, deleted=%d", createdRels, updatedRels, deletedRels)

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
		"Python graph updated: %d entities (+%d/-%d/~%d), %d relations (+%d/-%d/~%d)",
		len(entities), createdEntities, deletedEntities, updatedEntities,
		len(relations), createdRels, deletedRels, updatedRels)

	return
}

// RunPythonAnalyser runs the Python analyser script (python_analyser.py)
// on the given project directory. Honours PYTHON_ANALYSER_PATH and
// PYTHON_PATH environment variables for non-canonical installs.
func RunPythonAnalyser(projectDir string, outputDir string) ([]AnalyserResult, error) {
	interpreter := os.Getenv("PYTHON_PATH")
	if interpreter == "" {
		interpreter = "python"
	}

	script := os.Getenv("PYTHON_ANALYSER_PATH")
	if script == "" {
		script = findPythonAnalyser()
	}
	if script == "" {
		return nil, fmt.Errorf("python_analyser.py not found; set PYTHON_ANALYSER_PATH or ensure it is at coderag/cli/analysers/python/python_analyser.py")
	}

	if outputDir == "" {
		outputDir = filepath.Join(projectDir, ".coderag", "python_analyser_tmp")
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

	cmd := exec.Command(interpreter, script, "scan", "--dir", projectDir, "--output", outputDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running Python analyser on %s -> %s", projectDir, outputDir)
	status.Default.Stage("analyse", "running Python analyser")
	if err := cmd.Run(); err != nil {
		status.Default.Error("analyse", "Python analyser failed: %v", err)
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

	log.Printf("Python analyser produced %d output files", len(results))
	status.Default.Counts("analyse", map[string]int{"files": len(results)})
	return results, nil
}

// findPythonAnalyser locates python_analyser.py by probing repo-relative,
// executable-relative, and PYTHON_ANALYSER_PATH (set by the caller).
func findPythonAnalyser() string {
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

	for _, p := range []string{
		"coderag/cli/analysers/python/python_analyser.py",
		"./coderag/cli/analysers/python/python_analyser.py",
		"../coderag/cli/analysers/python/python_analyser.py",
	} {
		if f := find(p); f != "" {
			return f
		}
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		for _, dir := range []string{
			filepath.Join(exeDir, "analysers", "python"),
			filepath.Join(exeDir, "coderag", "cli", "analysers", "python"),
		} {
			c := filepath.Join(dir, "python_analyser.py")
			if f := find(c); f != "" {
				return f
			}
		}
	}

	if repo := repoRoot(); repo != "" {
		c := filepath.Join(repo, "coderag", "cli", "analysers", "python", "python_analyser.py")
		if f := find(c); f != "" {
			return f
		}
	}

	return ""
}

// findPythonProjectDir walks up from a .py file looking for a
// pyproject.toml marker. Falls back to the file's containing directory
// when nothing is found (the analyser will still walk it).
func findPythonProjectDir(filePath string) string {
	dir := filepath.Dir(filePath)
	for i := 0; i < 10; i++ {
		matches, _ := filepath.Glob(filepath.Join(dir, "pyproject.toml"))
		if len(matches) > 0 {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// No pyproject.toml — use the file's directory as the project root.
	return filepath.Dir(filePath)
}
