package pipeline

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/coderag/crud"
	"github.com/coderag/diff"
	"github.com/coderag/filter"
	"github.com/coderag/parser"
	"github.com/coderag/router"
	"github.com/coderag/status"
)

type dedupKey struct {
	src  string
	tgt  string
	kw   string
}

var (
	applyCSharpResultsMu sync.Mutex

	relationDedup   sync.Map
	dedupLastClean  int64
)

var httpClient = &http.Client{Timeout: 300}

type AnalyserResult struct {
	Filename string
	Content  string
}

func Run(filePath string, client *crud.Config) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	analyser := router.GetAnalyser(ext)
	if analyser == "" {
		status.Default.Detail("route", filePath, "no analyser for %s, LLM fallback", ext)
		return fallbackToLLM(filePath, client)
	}

	if analyser == "c_sharp" {
		status.Default.Detail("route", filePath, "routed to C# analyser")
		return runCSharpPipeline(filePath, client, nil)
	}

	status.Default.Detail("route", filePath, "no analyser for %s, LLM fallback", ext)
	return fallbackToLLM(filePath, client)
}

func RunEvent(filePath string, client *crud.Config, results []AnalyserResult) error {
	return RunEventWithChangeType(filePath, client, results, "")
}

func RunEventWithChangeType(filePath string, client *crud.Config, results []AnalyserResult, changeType string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	analyser := router.GetAnalyser(ext)
	if analyser == "" {
		status.Default.Detail("route", filePath, "no analyser for %s, LLM fallback", ext)
		return fallbackToLLM(filePath, client)
	}

	if analyser == "c_sharp" {
		status.Default.Detail("route", filePath, "routed to C# analyser (%s)", changeTypeOrCreated(changeType))
		if changeType == "modified" {
			return runCSharpPipelineModified(filePath, client, results)
		}
		return runCSharpPipelineWithResults(filePath, client, results)
	}

	status.Default.Detail("route", filePath, "no analyser for %s, LLM fallback", ext)
	return fallbackToLLM(filePath, client)
}

// changeTypeOrCreated normalizes an empty change type for status reporting.
func changeTypeOrCreated(changeType string) string {
	if changeType == "" {
		return "created"
	}
	return changeType
}

func runCSharpPipeline(filePath string, client *crud.Config, results []AnalyserResult) error {
	sln := findSolutionFile(filePath)
	if sln == "" {
		return fmt.Errorf("no .sln found for %s", filePath)
	}

	if results == nil {
		var err error
		results, err = RunCSharpAnalyser(sln, "")
		if err != nil {
			return fmt.Errorf("C# analyser failed: %w", err)
		}
	}

	if len(results) == 0 {
		return fmt.Errorf("C# analyser produced no output for %s", sln)
	}

	return applyCSharpResults(filePath, client, results)
}

func runCSharpPipelineWithResults(filePath string, client *crud.Config, results []AnalyserResult) error {
	sln := findSolutionFile(filePath)
	if sln == "" {
		return fmt.Errorf("no .sln found for %s", filePath)
	}

	if len(results) == 0 {
		return fmt.Errorf("no analyser results for %s", sln)
	}

	return applyCSharpResults(filePath, client, results)
}

func runCSharpPipelineModified(filePath string, client *crud.Config, results []AnalyserResult) error {
	log.Printf("Modified file detected: %s, refreshing entities", filePath)

	if err := client.DeleteEntitiesByFilePath(filePath); err != nil {
		log.Printf("Warning: failed to delete old entities for %s: %v", filePath, err)
	}

	if len(results) == 0 {
		sln := findSolutionFile(filePath)
		if sln == "" {
			return fmt.Errorf("no .sln found for %s", filePath)
		}
		var err error
		results, err = RunCSharpAnalyser(sln, "")
		if err != nil {
			return fmt.Errorf("C# analyser failed: %w", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("C# analyser produced no output for %s", sln)
		}
	}

	return runCSharpPipelineWithResults(filePath, client, results)
}

func applyCSharpResults(filePath string, client *crud.Config, results []AnalyserResult) error {
	applyCSharpResultsMu.Lock()
	defer applyCSharpResultsMu.Unlock()

	log.Printf("APPLY CSHARP RESULTS: processing %d results for %s", len(results), filePath)
	parserResults := make([]struct{ Filename string; Content string }, len(results))
	for i, r := range results {
		parserResults[i] = struct{ Filename string; Content string }{r.Filename, r.Content}
	}

	entities, relations := parser.ParseCSharpOutputWithConfig(parserResults, parser.LoadDuplicatesConfig(workspaceRoot()))

	log.Printf("Analyser extracted %d entities, %d relations from %s",
		len(entities), len(relations), filePath)

	currentEntities, err := client.GetAllEntities()
	if err != nil {
		currentEntities = []map[string]any{}
	}

	oldEntityMap := entitySliceToMap(currentEntities)
	newEntityMap := parserEntitiesToMap(entities)

	entityActions := diff.DiffEntities(oldEntityMap, newEntityMap)

	createdEntities := 0
	deletedEntities := 0
	updatedEntities := 0

	var entityBatch []crud.BatchEntityItem

	for _, action := range entityActions {
		switch action.Kind {
		case diff.ActionDelete:
			if _, err := client.DeleteEntity(action.EntityName); err != nil {
				log.Printf("ERROR deleting entity %q: %v", action.EntityName, err)
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
			entityType := getString(data, "entity_type", "")
			if entityType == "CLASS" {
				delete(data, "description")
				delete(data, "methods")
			}
			if _, err := client.UpdateEntity(action.EntityName, data); err != nil {
				log.Printf("ERROR updating entity %q: %v", action.EntityName, err)
			} else {
				updatedEntities++
			}
		}
	}

	oldRelMap := loadCurrentRelations(client)
	newRelMap := parserRelationsToMap(relations)
	relActions := diff.DiffRelations(oldRelMap, newRelMap)

	createdRels := 0
	updatedRels := 0
	deletedRels := 0

	var relBatch []crud.BatchRelationItem

	for _, action := range relActions {
		switch action.Kind {
		case diff.ActionDelete:
			if _, err := client.DeleteRelation(action.Source, action.Target); err != nil {
				log.Printf("ERROR deleting relation %q -> %q: %v", action.Source, action.Target, err)
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
			if _, err := client.UpdateRelation(action.Source, action.Target, data); err != nil {
				log.Printf("ERROR updating relation %q -> %q: %v", action.Source, action.Target, err)
			} else {
				updatedRels++
				log.Printf("RELATION UPDATE OK: %s -> %s", action.Source, action.Target)
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

	log.Printf("Entities (local): created=%d, updated=%d, deleted=%d", createdEntities, updatedEntities, deletedEntities)
	log.Printf("Relations (local): created=%d, updated=%d, deleted=%d", createdRels, updatedRels, deletedRels)

	// Report the batch result to the WebUI status endpoint (no-op when disabled)
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
		"graph updated: %d entities (+%d/-%d/~%d), %d relations (+%d/-%d/~%d)",
		len(entities), createdEntities, deletedEntities, updatedEntities,
		len(relations), createdRels, deletedRels, updatedRels)

	return nil
}

// workspaceRoot returns the configured workspace directory, or the
// scratch fallback when no env var is set. Used to locate per-project
// .coderag/duplicates.json so operators can override canonical-file
// selection without changing the binary.
func workspaceRoot() string {
	if v := os.Getenv("WORKSPACE_ROOT"); v != "" {
		return v
	}
	return ""
}

func fallbackToLLM(filePath string, client *crud.Config) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	versionedSource := getVersionedSource(filePath)
	status.Default.Detail("llm", filePath, "ingesting via LLM pipeline")
	return client.IngestText(string(content), versionedSource)
}

func RunCSharpAnalyser(sln string, outputDir string) ([]AnalyserResult, error) {
	analyserExe := os.Getenv("CSHARP_ANALYSER_PATH")
	if analyserExe == "" {
		analyserExe = findCSharpAnalyser()
	}
	if analyserExe == "" {
		return nil, fmt.Errorf("csharp_analyser not found; build it with 'cd coderag/cli/analysers/csharp && go build -o csharp_analyser.exe ./cmd/csharp_analyser/' or set CSHARP_ANALYSER_PATH")
	}

	if outputDir == "" {
		var err error
		outputDir, err = os.MkdirTemp("", "csharp_analyser")
		if err != nil {
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

	cmd := exec.Command(analyserExe, "scan", "--solution", sln, "--output", outputDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running C# analyser on %s -> %s", sln, outputDir)
	status.Default.Stage("analyse", "running C# analyser")
	if err := cmd.Run(); err != nil {
		status.Default.Error("analyse", "C# analyser failed: %v", err)
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
		// Skip _index.txt — global index file, not per-file output
		if entry.Name() == "_index.txt" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(outputDir, entry.Name()))
		if err != nil {
			continue
		}
		results = append(results, AnalyserResult{entry.Name(), string(content)})
	}

	log.Printf("Analyser produced %d output files", len(results))
	status.Default.Counts("analyse", map[string]int{"files": len(results)})
	return results, nil
}

func findSolutionFile(filePath string) string {
	dir := filepath.Dir(filePath)
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

func findCSharpAnalyser() string {
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
			filepath.Join(dir, "csharp_analyser.exe"),
			filepath.Join(dir, "csharp_analyser"),
		}
	}

	for _, p := range []string{
		"coderag/cli/analysers/csharp/csharp_analyser.exe",
		"coderag/cli/analysers/csharp/csharp_analyser",
		"./coderag/cli/analysers/csharp/csharp_analyser.exe",
		"./coderag/cli/analysers/csharp/csharp_analyser",
		"../coderag/cli/analysers/csharp/csharp_analyser.exe",
		"../coderag/cli/analysers/csharp/csharp_analyser",
	} {
		if f := find(p); f != "" {
			return f
		}
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		for _, dir := range []string{
			filepath.Join(exeDir, "analysers", "csharp"),
			filepath.Join(exeDir, "coderag", "cli", "analysers", "csharp"),
		} {
			for _, c := range names(dir) {
				if f := find(c); f != "" {
					return f
				}
			}
		}
	}

	if repo := repoRoot(); repo != "" {
		for _, c := range names(filepath.Join(repo, "coderag", "cli", "analysers", "csharp")) {
			if f := find(c); f != "" {
				return f
			}
		}
	}

	return ""
}

// repoRoot locates the codeRAG repository (containing the Dockerfile and
// coderag/lightrag) by walking up from the working directory and the
// executable directory.
func repoRoot() string {
	roots := []string{}
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	for _, start := range roots {
		for d := start; ; d = filepath.Dir(d) {
			if _, err := os.Stat(filepath.Join(d, "Dockerfile")); err == nil {
				if _, err := os.Stat(filepath.Join(d, "coderag", "lightrag")); err == nil {
					return d
				}
			}
			if filepath.Dir(d) == d {
				break
			}
		}
	}
	return ""
}

func entitySliceToMap(entities []map[string]any) map[string]map[string]any {
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

func parserEntitiesToMap(entities []parser.Entity) map[string]map[string]any {
	m := make(map[string]map[string]any)
	for _, e := range entities {
		row := map[string]any{
			"entity_type": e.EntityType,
			"description": e.Description,
			"source_id":   e.SourceID,
			"file_path":   e.FilePath,
		}
		// Spans are optional in legacy rows; abatch_import will pass them
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

func parserRelationsToMap(relations []parser.Relation) map[string]map[string]any {
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

func loadCurrentRelations(client *crud.Config) map[string]map[string]any {
	result := make(map[string]map[string]any)

	req, err := http.NewRequest("GET", client.BaseURL+"/graphs?label=*&max_depth=2&max_nodes=10000", nil)
	if err != nil {
		return result
	}
	for k, v := range client.Headers() {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
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

type versionedSource struct {
	count map[string]int
}

func (v *versionedSource) get(filePath string, isNew bool) string {
	if isNew {
		v.count[filePath] = 0
		return filePath
	}
	v.count[filePath]++
	return fmt.Sprintf("%s#v%d", filePath, v.count[filePath])
}

var _versioned = &versionedSource{count: make(map[string]int)}

func getVersionedSource(filePath string) string {
	return _versioned.get(filePath, false)
}