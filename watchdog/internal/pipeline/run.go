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

	"github.com/code-lightrag/watchdog/internal/crud"
	"github.com/code-lightrag/watchdog/internal/diff"
	"github.com/code-lightrag/watchdog/internal/filter"
	"github.com/code-lightrag/watchdog/internal/parser"
	"github.com/code-lightrag/watchdog/internal/router"
)

type dedupKey struct {
	src  string
	tgt  string
}

var (
	relationDedup   sync.Map
	dedupLastClean  int64
)

var httpClient = &http.Client{Timeout: 30}

type AnalyserResult struct {
	Filename string
	Content  string
}

func Run(filePath string, client *crud.Config) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	analyser := router.GetAnalyser(ext)
	if analyser == "" {
		return fallbackToLLM(filePath, client)
	}

	if analyser == "c_sharp" {
		return runCSharpPipeline(filePath, client, nil)
	}

	return fallbackToLLM(filePath, client)
}

func RunEvent(filePath string, client *crud.Config, results []AnalyserResult) error {
	return RunEventWithChangeType(filePath, client, results, "")
}

func RunEventWithChangeType(filePath string, client *crud.Config, results []AnalyserResult, changeType string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	analyser := router.GetAnalyser(ext)
	if analyser == "" {
		return fallbackToLLM(filePath, client)
	}

	if analyser == "c_sharp" {
		if changeType == "modified" {
			return runCSharpPipelineModified(filePath, client, results)
		}
		return runCSharpPipelineWithResults(filePath, client, results)
	}

	return fallbackToLLM(filePath, client)
}

func runCSharpPipeline(filePath string, client *crud.Config, results []AnalyserResult) error {
	sln := findSolutionFile(filePath)
	if sln == "" {
		return fmt.Errorf("no .sln found for %s", filePath)
	}

	if results == nil {
		var err error
		results, err = RunCSharpAnalyser(sln)
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

	return runCSharpPipelineWithResults(filePath, client, results)
}

func applyCSharpResults(filePath string, client *crud.Config, results []AnalyserResult) error {
	log.Printf("APPLY CSHARP RESULTS: processing %d results for %s", len(results), filePath)
	parserResults := make([]struct{ Filename string; Content string }, len(results))
	for i, r := range results {
		parserResults[i] = struct{ Filename string; Content string }{r.Filename, r.Content}
	}

	entities, relations := parser.ParseCSharpOutput(parserResults)

	log.Printf("Analyser extracted %d entities, %d relations from %s",
		len(entities), len(relations), filePath)

	currentEntities, err := client.GetAllEntities()
	if err != nil {
		currentEntities = []map[string]any{}
	}

	oldEntityMap := entitySliceToMap(currentEntities)
	newEntityMap := parserEntitiesToMap(entities)

	entityActions := diff.DiffEntities(oldEntityMap, newEntityMap)

	for _, action := range entityActions {
		switch action.Kind {
		case diff.ActionDelete:
			client.DeleteEntity(action.EntityName)
		case diff.ActionCreate:
			data := filter.FilterEntityData(action.Data)
			_, err := client.CreateEntity(
				action.EntityName,
				filter.FilterDescription(getString(data, "description")),
				getString(data, "entity_type", "UNKNOWN"),
				getString(data, "source_id", filePath),
				getString(data, "file_path", filePath),
			)
			if err != nil {
				log.Printf("Entity create error: %v", err)
			}
		case diff.ActionUpdate:
			data := filter.FilterEntityData(action.Data)
			entityType := getString(data, "entity_type", "")
			if entityType == "CLASS" {
				log.Printf("Skipping CLASS update for %s (description auto-generated)", action.EntityName)
				continue
			}
			client.UpdateEntity(action.EntityName, data)
		}
	}

	oldRelMap := loadCurrentRelations(client)
	newRelMap := parserRelationsToMap(relations)
	relActions := diff.DiffRelations(oldRelMap, newRelMap)

	for _, action := range relActions {
		switch action.Kind {
		case diff.ActionDelete:
			client.DeleteRelation(action.Source, action.Target)
		case diff.ActionCreate:
			key := dedupKey{src: action.Source, tgt: action.Target}
			if _, loaded := relationDedup.LoadOrStore(key, true); loaded {
				log.Printf("RELATION SKIP (dedup): %s -> %s", action.Source, action.Target)
				continue
			}

			data := filter.FilterRelationData(action.Data)
			desc := filter.FilterDescription(getString(data, "description"))
			kw := getString(data, "keywords")
			w := getFloat(data, "weight", 1.0)
			sid := getString(data, "source_id", filePath)
			fp := getString(data, "file_path", filePath)
			log.Printf("RELATION CREATE: %s -> %s desc=%q kw=%q weight=%.1f",
				action.Source, action.Target, desc, kw, w)
			resp, err := client.CreateRelation(
				action.Source,
				action.Target,
				desc,
				kw,
				w,
				sid,
				fp,
			)
			if err != nil {
				log.Printf("RELATION CREATE ERROR: %v", err)
			} else {
				log.Printf("RELATION CREATE OK: %v", resp)
			}
		case diff.ActionUpdate:
			data := filter.FilterRelationData(action.Data)
			client.UpdateRelation(action.Source, action.Target, data)
		}
	}

	return nil
}

func fallbackToLLM(filePath string, client *crud.Config) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	versionedSource := getVersionedSource(filePath)
	return client.IngestText(string(content), versionedSource)
}

func RunCSharpAnalyser(sln string) ([]AnalyserResult, error) {
	analyserExe := os.Getenv("CSHARP_ANALYSER_PATH")
	if analyserExe == "" {
		analyserExe = findCSharpAnalyser()
	}
	if analyserExe == "" {
		return nil, fmt.Errorf("CSharpAnalyzerTool.exe not found")
	}

	tempDir, err := os.MkdirTemp("", "csharp_analyser")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.Command(analyserExe, sln, tempDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running C# analyser on %s -> %s", sln, tempDir)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run analyser: %w", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return nil, fmt.Errorf("read temp dir: %w", err)
	}

	var results []AnalyserResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(tempDir, entry.Name()))
		if err != nil {
			continue
		}
		results = append(results, AnalyserResult{entry.Name(), string(content)})
	}

	log.Printf("Analyser produced %d output files", len(results))
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
	paths := []string{
		"daniel-lightrag-mcp/analysers/c_sharp/CSharpAnalyzerTool.exe",
		"./daniel-lightrag-mcp/analysers/c_sharp/CSharpAnalyzerTool.exe",
		"../daniel-lightrag-mcp/analysers/c_sharp/CSharpAnalyzerTool.exe",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
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
		m[e.Name] = map[string]any{
			"entity_type": e.EntityType,
			"description": e.Description,
			"source_id":   e.SourceID,
			"file_path":   e.FilePath,
			"methods":     e.Methods,
		}
	}
	return m
}

func parserRelationsToMap(relations []parser.Relation) map[string]map[string]any {
	result := make(map[string]map[string]any)
	for _, r := range relations {
		key := diff.RelationKey(r.Source, r.Target)
		result[key] = map[string]any{
			"description": r.Description,
			"keywords":    r.Keywords,
			"weight":      r.Weight,
		}
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

		if s, ok := edge["source"].(string); ok && s != "" {
			src = s
		}
		if t, ok := edge["target"].(string); ok && t != "" {
			tgt = t
		}

		if src == "" || tgt == "" {
			continue
		}

		key := diff.RelationKey(src, tgt)
		result[key] = edge
	}
	return result
}

func getString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
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