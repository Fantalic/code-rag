package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/coderag/crud"
	"github.com/coderag/diff"
	"github.com/coderag/filter"
	"github.com/coderag/parser"
	"github.com/coderag/pipeline"
	"github.com/coderag/projects"
	"github.com/coderag/registry"
	"github.com/coderag/status"
	"github.com/urfave/cli/v2"
)

// ── Project registry helpers ──────────────────────────────────────────────
//
// Each scan/watch session upserts the project in the host-CLI registry
// (persistent JSON file) AND mirrors it to the LightRAG server so the
// WebUI shows the project in its switcher.

func scanDeltaCounts(s scanStats) map[string]int {
	return map[string]int{
		"files":             s.Files,
		"entities_created":  s.EntitiesCreated,
		"entities_updated":  s.EntitiesUpdated,
		"entities_deleted":  s.EntitiesDeleted,
		"relations_created": s.RelationsCreated,
		"relations_updated": s.RelationsUpdated,
		"relations_deleted": s.RelationsDeleted,
	}
}

func pushProjectStart(inputDir, ws, kind, baseURL string) {
	reg := registry.New(registry.DefaultPath())
	p := registry.Project{
		Path:       inputDir,
		Name:       ws,
		Kind:       kind,
		LastScanAt: time.Now().Unix(),
		LastStatus: "scanning",
	}
	saved, err := reg.Upsert(p, nil)
	if err != nil {
		log.Printf("Warning: registry upsert failed: %v", err)
		return
	}
	_ = reg.SetFocused(saved.ID)
	pc := projects.New(baseURL)
	defer pc.Close()
	if err := pc.Push(saved, true); err != nil {
		log.Printf("Warning: project push to server failed: %v", err)
	}
	if err := pc.FocusPOST(saved.ID); err != nil {
		log.Printf("Warning: project focus push failed: %v", err)
	}
}

// pushProjectDone publishes final project state to the host registry and
// the LightRAG mirror. Safe to call multiple times (idempotent upserts).
func pushProjectDone(ws, inputDir, kind, baseURL string, s scanStats, finalStatus, msg string) error {
	reg := registry.New(registry.DefaultPath())
	p := registry.Project{
		Path:        inputDir,
		Name:        ws,
		Kind:        kind,
		LastScanAt:  time.Now().Unix(),
		LastStatus:  finalStatus,
		LastMessage: msg,
	}
	if finalStatus == "error" {
		p.ErrorsTotal = 1
	}
	saved, err := reg.Upsert(p, scanDeltaCounts(s))
	if err != nil {
		return err
	}
	pc := projects.New(baseURL)
	defer pc.Close()
	if err := pc.Push(saved, true); err != nil {
		log.Printf("Warning: project done push failed: %v", err)
	}
	return nil
}

// ── scan subcommand ───────────────────────────────────────────────────────

func scanCmd(c *cli.Context) error {
	inputDir := inputDirOrDefault(c)
	if _, err := os.Stat(inputDir); err != nil {
		return fmt.Errorf("directory not found: %s", inputDir)
	}

	ws := projectName(inputDir)
	clean := c.Bool("clean")

	ctx := context.Background()
	if err := ensureLightRAG(ctx, ws); err != nil {
		return fmt.Errorf("failed to start LightRAG: %w", err)
	}

	coderagDir := filepath.Join(inputDir, ".coderag")
	analyserOut := filepath.Join(coderagDir, "analyser_output")

	if clean {
		log.Printf("Clearing analyser output directory...")
		os.RemoveAll(analyserOut)

		log.Printf("Clearing existing data from LightRAG...")
		clearClient := crud.NewConfig()
		if err := clearClient.SetWorkspace(ws); err != nil {
			log.Printf("Warning: failed to set workspace for clear: %v", err)
		}
		if err := clearClient.ClearAll(); err != nil {
			return fmt.Errorf("failed to clear LightRAG: %w", err)
		}
		time.Sleep(3 * time.Second)
	}

	logsDir := filepath.Join(coderagDir, "docker-logs")
	_ = captureDockerLogs(logsDir)

	client := crud.NewConfig()

	// Install the process-wide status reporter so pipeline/watcher hooks
	// report progress to the WebUI (no-op when CODERAG_STATUS_DISABLE=1)
	rep := status.New(client.BaseURL)
	defer rep.Close()
	restoreRep := status.SetDefault(rep)
	defer restoreRep()

	// Dispatch by project type: .sln → C#, build.zig → Zig, pyproject.toml → Python.
	// The whole scan path (analyser invocation, parser, LightRAG ingest) flows
	// through the `coderag` CLI binary — no separate executables need
	// to be invoked by the user.
	sln := findSolutionFile(inputDir)
	if sln != "" {
		return scanCSharpProject(inputDir, ws, sln, analyserOut, clean, client, rep)
	}

	zigRoot := findZigBuildFile(inputDir)
	if zigRoot != "" {
		return scanZigProject(inputDir, ws, zigRoot, analyserOut, clean, client, rep)
	}

	pyProject := findPyProjectToml(inputDir)
	if pyProject != "" {
		return scanPythonProject(inputDir, ws, pyProject, analyserOut, clean, client, rep)
	}

	rep.Error("scan", "no .sln, build.zig, or pyproject.toml found in %s", inputDir)
	return fmt.Errorf("no .sln, build.zig, or pyproject.toml found in %s — codeRAG supports C# solutions, Zig projects (build.zig), and Python projects (pyproject.toml)", inputDir)
}

// scanCSharpProject is the original C# scan path: one .sln → one
// RunCSharpAnalyser call → one applyScanResults call.
func scanCSharpProject(inputDir, ws, sln, analyserOut string, clean bool, client *crud.Config, rep *status.Reporter) error {
	rep.Stage("scan", "scan started (csharp): %s%s", inputDir, map[bool]string{true: " (--clean)", false: ""}[clean])
	pushProjectStart(inputDir, ws, "csharp", client.BaseURL)

	log.Printf("Setting workspace to '%s' for project %s", ws, inputDir)
	if err := client.SetWorkspace(ws); err != nil {
		log.Printf("Warning: failed to set workspace: %v (continuing with default)", err)
	}

	log.Printf("Found solution: %s", sln)
	rep.Detail("analyse", sln, "solution found")

	log.Printf("Running C# analyser...")
	results, err := pipeline.RunCSharpAnalyser(sln, analyserOut)
	if err != nil {
		rep.Error("analyse", "C# analyser failed: %v", err)
		return fmt.Errorf("C# analyser failed: %w", err)
	}
	if len(results) == 0 {
		rep.Error("analyse", "C# analyser produced no output for %s", sln)
		return fmt.Errorf("C# analyser produced no output for %s", sln)
	}
	log.Printf("Analyser produced %d output files", len(results))
	rep.Counts("analyse", map[string]int{"files": len(results)})

	log.Printf("Applying results to knowledge graph...")
	scan, err := applyScanResults(client, results)
	if err != nil {
		rep.Error("crud", "failed to apply scan results: %v", err)
		return fmt.Errorf("failed to apply scan results: %w", err)
	}

	stats, err := client.GetAllEntities()
	if err != nil {
		log.Printf("Scan complete (could not get entity count: %v)", err)
		rep.Done("scan complete")
	} else {
		log.Printf("Scan complete. Knowledge graph now contains %d entities", len(stats))
		rep.Done("scan complete: knowledge graph now contains %d entities", len(stats))
	}

	if err := pushProjectDone(ws, inputDir, "csharp", client.BaseURL, scan, "idle", fmt.Sprintf("scan complete: %d entities", len(stats))); err != nil {
		log.Printf("Warning: failed to publish project state: %v", err)
	}

	return nil
}

// scanZigProject is the Zig scan path: build.zig marks the project root
// → one RunZigAnalyser call walks the whole project → one
// applyZigResults call ingests all .txt sidecars into LightRAG.
func scanZigProject(inputDir, ws, projectDir, analyserOut string, clean bool, client *crud.Config, rep *status.Reporter) error {
	rep.Stage("scan", "scan started (zig): %s%s", inputDir, map[bool]string{true: " (--clean)", false: ""}[clean])
	pushProjectStart(inputDir, ws, "zig", client.BaseURL)

	log.Printf("Setting workspace to '%s' for project %s", ws, inputDir)
	if err := client.SetWorkspace(ws); err != nil {
		log.Printf("Warning: failed to set workspace: %v (continuing with default)", err)
	}

	log.Printf("Found Zig project root: %s", projectDir)
	rep.Detail("analyse", projectDir, "Zig project root")

	log.Printf("Running Zig analyser...")
	results, err := pipeline.RunZigAnalyser(projectDir, analyserOut)
	if err != nil {
		rep.Error("analyse", "Zig analyser failed: %v", err)
		return fmt.Errorf("Zig analyser failed: %w", err)
	}
	if len(results) == 0 {
		rep.Error("analyse", "Zig analyser produced no output for %s", projectDir)
		return fmt.Errorf("Zig analyser produced no output for %s", projectDir)
	}
	log.Printf("Zig analyser produced %d output files", len(results))
	rep.Counts("analyse", map[string]int{"files": len(results)})

	log.Printf("Applying results to knowledge graph...")
	if err := pipeline.ApplyZigScanResults(client, projectDir, results); err != nil {
		rep.Error("crud", "failed to apply Zig scan results: %v", err)
		return fmt.Errorf("failed to apply Zig scan results: %w", err)
	}

	stats, err := client.GetAllEntities()
	if err != nil {
		log.Printf("Scan complete (could not get entity count: %v)", err)
		rep.Done("scan complete")
	} else {
		log.Printf("Scan complete. Knowledge graph now contains %d entities", len(stats))
		rep.Done("scan complete: knowledge graph now contains %d entities", len(stats))
	}

	// Build a minimal scanStats for the project registry (counts come
	// from the analyse/apply logs above).
	scan := scanStats{
		Files:           len(results),
		EntitiesCreated: len(stats),
	}
	if err := pushProjectDone(ws, inputDir, "zig", client.BaseURL, scan, "idle", fmt.Sprintf("scan complete: %d entities", len(stats))); err != nil {
		log.Printf("Warning: failed to publish project state: %v", err)
	}

	return nil
}

// scanPythonProject is the Python scan path: pyproject.toml marks the project root
// → one RunPythonAnalyser call walks the whole project → one
// ApplyPythonScanResults call ingests all .txt sidecars into LightRAG.
func scanPythonProject(inputDir, ws, projectDir, analyserOut string, clean bool, client *crud.Config, rep *status.Reporter) error {
	rep.Stage("scan", "scan started (python): %s%s", inputDir, map[bool]string{true: " (--clean)", false: ""}[clean])
	pushProjectStart(inputDir, ws, "python", client.BaseURL)

	log.Printf("Setting workspace to '%s' for project %s", ws, inputDir)
	if err := client.SetWorkspace(ws); err != nil {
		log.Printf("Warning: failed to set workspace: %v (continuing with default)", err)
	}

	log.Printf("Found Python project root: %s", projectDir)
	rep.Detail("analyse", projectDir, "Python project root")

	log.Printf("Running Python analyser...")
	results, err := pipeline.RunPythonAnalyser(projectDir, analyserOut)
	if err != nil {
		rep.Error("analyse", "Python analyser failed: %v", err)
		return fmt.Errorf("Python analyser failed: %w", err)
	}
	if len(results) == 0 {
		rep.Error("analyse", "Python analyser produced no output for %s", projectDir)
		return fmt.Errorf("Python analyser produced no output for %s", projectDir)
	}
	log.Printf("Python analyser produced %d output files", len(results))
	rep.Counts("analyse", map[string]int{"files": len(results)})

	log.Printf("Applying results to knowledge graph...")
	if err := pipeline.ApplyPythonScanResults(client, projectDir, results); err != nil {
		rep.Error("crud", "failed to apply Python scan results: %v", err)
		return fmt.Errorf("failed to apply Python scan results: %w", err)
	}

	stats, err := client.GetAllEntities()
	if err != nil {
		log.Printf("Scan complete (could not get entity count: %v)", err)
		rep.Done("scan complete")
	} else {
		log.Printf("Scan complete. Knowledge graph now contains %d entities", len(stats))
		rep.Done("scan complete: knowledge graph now contains %d entities", len(stats))
	}

	// Build a minimal scanStats for the project registry (counts come
	// from the analyse/apply logs above).
	scan := scanStats{
		Files:           len(results),
		EntitiesCreated: len(stats),
	}
	if err := pushProjectDone(ws, inputDir, "python", client.BaseURL, scan, "idle", fmt.Sprintf("scan complete: %d entities", len(stats))); err != nil {
		log.Printf("Warning: failed to publish project state: %v", err)
	}

	return nil
}

func findSolutionFile(inputDir string) string {
	dir := inputDir
	for i := 0; i < 10; i++ {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.sln"))
		if len(matches) > 0 {
			return matches[0]
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// findZigBuildFile walks up from inputDir looking for a build.zig
// marker that identifies the Zig project root. Returns "" if none
// found within 10 levels.
func findZigBuildFile(inputDir string) string {
	dir := inputDir
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
	return ""
}

// findPyProjectToml walks up from inputDir looking for a pyproject.toml
// marker that identifies the Python project root. Returns "" if none
// found within 10 levels.
func findPyProjectToml(inputDir string) string {
	dir := inputDir
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
	return ""
}

type scanStats struct {
	Files            int
	EntitiesCreated  int
	EntitiesUpdated  int
	EntitiesDeleted  int
	RelationsCreated int
	RelationsUpdated int
	RelationsDeleted int
}

func applyScanResults(client *crud.Config, results []pipeline.AnalyserResult) (scanStats, error) {
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

	entities, relations := parser.ParseCSharpOutput(parserResults)
	log.Printf("Extracted %d entities, %d relations", len(entities), len(relations))

	currentEntities, err := client.GetAllEntities()
	if err != nil {
		currentEntities = []map[string]any{}
	}

	oldEntityMap := entitySliceToMapScan(currentEntities)
	newEntityMap := parserEntitiesToMapScan(entities)

	entityActions := diff.DiffEntities(oldEntityMap, newEntityMap)

	createdEntities := 0
	updatedEntities := 0
	deletedEntities := 0

	var entityBatch []crud.BatchEntityItem
	var stats scanStats
	stats.Files = len(results)

	for _, action := range entityActions {
		switch action.Kind {
		case diff.ActionDelete:
			if _, err := client.DeleteEntity(action.EntityName); err == nil {
				deletedEntities++
			} else {
				log.Printf("ERROR deleting entity %q: %v", action.EntityName, err)
			}
		case diff.ActionCreate:
			data := filter.FilterEntityData(action.Data)
			entityBatch = append(entityBatch, crud.BatchEntityItem{
				EntityName: action.EntityName,
				EntityData: crud.EntityData{
					Description: filter.FilterDescription(getString(data, "description")),
					EntityType:  getString(data, "entity_type", "UNKNOWN"),
					SourceID:    getString(data, "source_id", ""),
					FilePath:    getString(data, "file_path", ""),
				},
			})
		case diff.ActionUpdate:
			data := filter.FilterEntityData(action.Data)
			entityType := getString(data, "entity_type", "")
			if entityType == "CLASS" {
				delete(data, "description")
				delete(data, "methods")
			}
			if _, err := client.UpdateEntity(action.EntityName, data); err == nil {
				updatedEntities++
			} else {
				log.Printf("ERROR updating entity %q: %v", action.EntityName, err)
			}
		}
	}

	oldRelMap := loadCurrentRelationsScan(client)
	newRelMap := parserRelationsToMapScan(relations)
	relActions := diff.DiffRelations(oldRelMap, newRelMap)

	createdRels := 0
	updatedRels := 0
	deletedRels := 0

	var relBatch []crud.BatchRelationItem

	for _, action := range relActions {
		switch action.Kind {
		case diff.ActionDelete:
			if _, err := client.DeleteRelation(action.Source, action.Target); err == nil {
				deletedRels++
			} else {
				log.Printf("ERROR deleting relation %q -> %q: %v", action.Source, action.Target, err)
			}
		case diff.ActionCreate:
			data := filter.FilterRelationData(action.Data)
			relBatch = append(relBatch, crud.BatchRelationItem{
				SourceEntity: action.Source,
				TargetEntity: action.Target,
				RelationData: crud.RelationData{
					Description: filter.FilterDescription(getString(data, "description")),
					Keywords:    getString(data, "keywords"),
					Weight:      getFloat(data, "weight", 1.0),
					SourceID:    getString(data, "source_id", ""),
					FilePath:    getString(data, "file_path", ""),
				},
			})
		case diff.ActionUpdate:
			data := filter.FilterRelationData(action.Data)
			if _, err := client.UpdateRelation(action.Source, action.Target, data); err == nil {
				updatedRels++
			} else {
				log.Printf("ERROR updating relation %q -> %q: %v", action.Source, action.Target, err)
			}
		}
	}

	if len(entityBatch) > 0 || len(relBatch) > 0 {
		result, err := client.BatchImport(entityBatch, relBatch)
		if err == nil {
			if data, ok := result["data"].(map[string]any); ok {
				if c, ok := data["entities_created"].(float64); ok {
					createdEntities += int(c)
				}
				if c, ok := data["relations_created"].(float64); ok {
					createdRels += int(c)
				}
			}
		} else {
			log.Printf("ERROR batch import: %v", err)
		}
	}

	log.Printf("Entities: created=%d, updated=%d, deleted=%d", createdEntities, updatedEntities, deletedEntities)
	log.Printf("Relations: created=%d, updated=%d, deleted=%d", createdRels, updatedRels, deletedRels)

	stats = scanStats{
		Files:            len(results),
		EntitiesCreated:  createdEntities,
		EntitiesUpdated:  updatedEntities,
		EntitiesDeleted:  deletedEntities,
		RelationsCreated: createdRels,
		RelationsUpdated: updatedRels,
		RelationsDeleted: deletedRels,
	}

	// Report scan totals to the WebUI status endpoint (no-op when disabled)
	status.Default.Counts("crud", map[string]int{
		"entities_created":  stats.EntitiesCreated,
		"entities_updated":  stats.EntitiesUpdated,
		"entities_deleted":  stats.EntitiesDeleted,
		"relations_created": stats.RelationsCreated,
		"relations_updated": stats.RelationsUpdated,
		"relations_deleted": stats.RelationsDeleted,
	})

	return stats, nil
}

func entitySliceToMapScan(entities []map[string]any) map[string]map[string]any {
	m := make(map[string]map[string]any)
	for _, e := range entities {
		name := ""
		if n, ok := e["id"].(string); ok && n != "" {
			name = n
		} else if props, ok := e["properties"].(map[string]any); ok {
			if n, ok := props["entity_name"].(string); ok && n != "" {
				name = n
			} else if n, ok := props["entity_id"].(string); ok && n != "" {
				name = n
			}
		}
		if name != "" {
			if props, ok := e["properties"].(map[string]any); ok {
				m[name] = props
			} else {
				m[name] = e
			}
		}
	}
	return m
}

func parserEntitiesToMapScan(entities []parser.Entity) map[string]map[string]any {
	m := make(map[string]map[string]any)
	for _, e := range entities {
		row := map[string]any{
			"entity_type": e.EntityType,
			"description": e.Description,
			"source_id":   e.SourceID,
			"file_path":   e.FilePath,
		}
		// Spans are optional in legacy rows; abatch_import passes them
		// through only when both ends are present.
		if e.StartLine > 0 {
			row["start_line"] = e.StartLine
		}
		if e.EndLine > 0 {
			row["end_line"] = e.EndLine
		}
		m[e.Name] = row
	}
	return m
}

func parserRelationsToMapScan(relations []parser.Relation) map[string]map[string]any {
	result := make(map[string]map[string]any)
	for _, r := range relations {
		key := diff.RelationKeyIdent(r.Source, r.Target, r.Keywords)
		m := map[string]any{
			"description": r.Description,
			"keywords":    r.Keywords,
			"weight":      r.Weight,
			"source":      r.Source,
			"target":      r.Target,
		}
		if r.SourceID != "" {
			m["source_id"] = r.SourceID
		}
		if r.FilePath != "" {
			m["file_path"] = r.FilePath
		}
		result[key] = m
	}
	return result
}

func loadCurrentRelationsScan(client *crud.Config) map[string]map[string]any {
	result := make(map[string]map[string]any)

	req, err := http.NewRequest("GET", client.BaseURL+"/graphs?label=*&max_depth=2&max_nodes=10000", nil)
	if err != nil {
		return result
	}
	for k, v := range client.Headers() {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	var data struct {
		Edges []map[string]any `json:"edges"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return result
	}

	for _, edge := range data.Edges {
		src := ""
		tgt := ""
		kw := ""

		if s, ok := edge["source"].(string); ok && s != "" {
			src = s
		}
		if t, ok := edge["target"].(string); ok && t != "" {
			tgt = t
		}
		if props, ok := edge["properties"].(map[string]any); ok {
			if k, ok := props["keywords"].(string); ok {
				kw = k
			}
		}

		if src == "" || tgt == "" {
			continue
		}

		key := diff.RelationKeyIdent(src, tgt, kw)
		result[key] = diff.NormalizeRelationData(edge)
	}
	return result
}

func getString(m map[string]any, key string, defaultVal ...string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return ""
}

func getFloat(m map[string]any, key string, defaultVal float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return defaultVal
}
