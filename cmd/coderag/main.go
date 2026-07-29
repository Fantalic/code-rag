package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/coderag/pkg/watchdog/crud"
	"github.com/coderag/pkg/watchdog/diff"
	"github.com/coderag/pkg/watchdog/filter"
	"github.com/coderag/pkg/watchdog/pipeline"
	"github.com/coderag/pkg/watchdog/parser"
	"github.com/coderag/pkg/watchdog/watcher"
)

func main() {
	app := &cli.App{
		Name:  "coderag",
		Usage: "codeRAG — knowledge graph for codebases",
		Commands: []*cli.Command{
			{
				Name:   "start",
				Usage:  "Start Docker services + file watcher",
				Action: startCmd,
				Args:   true,
			},
			{
				Name:   "mcp",
				Usage:  "Start MCP stdio↔SSE bridge for a project directory",
				Action: mcpCmd,
				Args:   true,
			},
		{
			Name:   "scan",
			Usage:  "Run C# analyser and populate knowledge graph",
			Action: scanCmd,
			Args:   true,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "clean",
					Aliases: []string{"c"},
					Usage:   "Clear all existing data in LightRAG before scanning",
				},
			},
		},
			{
				Name:   "webui",
				Usage:  "Open WebUI in browser",
				Action: webuiCmd,
			},
			{
				Name:   "watch",
				Usage:  "Start file watcher only",
				Action: watchCmd,
				Args:   true,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "url",
						Aliases: []string{"u"},
						Usage:   "LightRAG API URL",
						EnvVars: []string{"LIGHTRAG_URL"},
						Value:   "http://localhost:9621",
					},
					&cli.StringFlag{
						Name:    "debounce",
						Aliases: []string{"d"},
						Usage:   "Debounce delay in seconds",
						EnvVars: []string{"DEBOUNCE_SECONDS"},
						Value:   "0.5",
					},
					&cli.StringFlag{
						Name:    "workers",
						Aliases: []string{"w"},
						Usage:   "Number of worker threads",
						EnvVars: []string{"WATCHER_WORKERS"},
						Value:   "4",
					},
				},
			},
			{
				Name:   "status",
				Usage:  "Show running services status",
				Action: statusCmd,
			},
			{
				Name:   "stop",
				Usage:  "Stop all running services",
				Action: stopCmd,
			},
			{
				Name:   "query",
				Usage:  "Query the knowledge graph",
				Action: queryCmd,
				Args:   true,
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func ensureMCP(ctx context.Context, workspace string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:9622/health", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil && resp.StatusCode < 400 {
		resp.Body.Close()
		return nil
	}
	if resp != nil {
		resp.Body.Close()
	}

	log.Print("MCP not reachable at :9622, ensuring Docker services are up...")
	if err := ensureLightRAG(ctx, workspace); err != nil {
		return err
	}

	log.Print("Waiting for MCP SSE to become healthy...")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for MCP at http://localhost:9622/health")
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:9622/health", nil)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 400 {
					log.Print("MCP SSE is healthy")
					return nil
				}
			}
		}
	}
}

func ensureLightRAG(ctx context.Context, workspace string) error {
	// Check if LightRAG is already running with the correct workspace
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:9621/health", nil)
	if err == nil {
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode < 400 {
			var health struct {
				Configuration struct {
					Workspace string `json:"workspace"`
				} `json:"configuration"`
			}
			if json.NewDecoder(resp.Body).Decode(&health) == nil {
				resp.Body.Close()
				if workspace == "" || health.Configuration.Workspace == workspace {
					log.Printf("LightRAG is healthy (workspace: '%s')", health.Configuration.Workspace)
					return nil
				}
				log.Printf("Workspace mismatch: container has '%s', need '%s'", health.Configuration.Workspace, workspace)
			} else {
				resp.Body.Close()
			}
		} else if resp != nil {
			resp.Body.Close()
		}
	}

	// Need to restart with the correct workspace
	log.Printf("Starting Docker services with workspace '%s'...", workspace)

	composeFile := findComposeFile()

	// Stop existing containers first
	downCmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "down")
	downCmd.Stdout = os.Stdout
	downCmd.Stderr = os.Stderr
	_ = downCmd.Run()

	// Start with the correct workspace
	upCmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "up", "-d")
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	upCmd.Env = append(os.Environ(), "WORKSPACE="+workspace)
	if err := upCmd.Run(); err != nil {
		return fmt.Errorf("docker compose failed: %w", err)
	}

	log.Print("Waiting for LightRAG to become healthy...")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.After(120 * time.Second)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for LightRAG at http://localhost:9621/health")
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:9621/health", nil)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 400 {
					log.Print("LightRAG is healthy")
					// Save Docker logs after successful start
					coderagDir := filepath.Join(filepath.Dir(composeFile), ".coderag")
					_ = captureDockerLogs(filepath.Join(coderagDir, "docker-logs"))
					return nil
				}
			}
		}
	}
}

func findComposeFile() string {
	candidates := []string{
		"docker-compose.yml",
		"../docker-compose.yml",
		"./docker-compose.yml",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return "docker-compose.yml"
}

func inputDirOrDefault(c *cli.Context) string {
	inputDir := c.Args().First()
	if inputDir == "" {
		var err error
		inputDir, err = os.Getwd()
		if err != nil {
			log.Fatalf("get current directory: %v", err)
		}
	}
	abs, err := filepath.Abs(inputDir)
	if err == nil {
		inputDir = abs
	}
	return inputDir
}

func startCmd(c *cli.Context) error {
	inputDir := inputDirOrDefault(c)
	if _, err := os.Stat(inputDir); err != nil {
		return fmt.Errorf("directory not found: %s", inputDir)
	}

	ws := projectName(inputDir)
	ctx := context.Background()
	if err := ensureLightRAG(ctx, ws); err != nil {
		return fmt.Errorf("failed to start LightRAG: %w", err)
	}

	client := crud.NewConfig()
	log.Printf("Setting workspace to '%s' for project %s", ws, inputDir)
	if err := client.SetWorkspace(ws); err != nil {
		log.Printf("Warning: failed to set workspace: %v (continuing with default)", err)
	}

	cfg := &watcher.Config{
		InputDir:     inputDir,
		DebounceSecs: 0.5,
		MaxWorkers:   4,
		UseGit:       true,
		LightRAGConfig: crud.NewConfig(),
	}

	w, err := watcher.NewWatcher(cfg)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	if err := w.Start(); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	log.Printf("Watching %s (press Ctrl+C to stop)", inputDir)
	log.Printf("WebUI: http://localhost:9621")

	sigCh := make(chan os.Signal, 1)
	<-sigCh

	w.Stop()
	return nil
}

func webuiCmd(c *cli.Context) error {
	req, err := http.Get("http://localhost:9621/health")
	if err != nil || req.StatusCode >= 400 {
		if req != nil {
			req.Body.Close()
		}
		return fmt.Errorf("LightRAG not running. Run 'coderag start <dir>' first.")
	}
	req.Body.Close()

	log.Print("Opening http://localhost:9621 in browser...")
	return openBrowser("http://localhost:9621")
}

func openBrowser(url string) error {
	switch {
	case len(os.Getenv("WSL_DISTRO_NAME")) > 0:
		return exec.Command("wslview", url).Start()
	default:
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}

func watchCmd(c *cli.Context) error {
	inputDir := inputDirOrDefault(c)
	if _, err := os.Stat(inputDir); err != nil {
		return fmt.Errorf("directory not found: %s", inputDir)
	}

	lightragURL := c.String("url")

	debounce, _ := strconv.ParseFloat(c.String("debounce"), 64)
	if debounce <= 0 {
		debounce = 0.5
	}

	workers, _ := strconv.Atoi(c.String("workers"))
	if workers <= 0 {
		workers = 4
	}

	cfg := &watcher.Config{
		InputDir:     inputDir,
		DebounceSecs: debounce,
		MaxWorkers:   workers,
		UseGit:       true,
		LightRAGConfig: &crud.Config{
			BaseURL: lightragURL,
			Timeout: 30 * time.Second,
		},
	}

	w, err := watcher.NewWatcher(cfg)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	if err := w.Start(); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	log.Printf("Watching %s (press Ctrl+C to stop)", inputDir)

	sigCh := make(chan os.Signal, 1)
	<-sigCh

	w.Stop()
	return nil
}

func statusCmd(c *cli.Context) error {
	client := crud.NewConfig()

	req, err := http.NewRequest("GET", client.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	for k, v := range client.Headers() {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("LightRAG: not reachable at %s", client.BaseURL)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode < 400 {
			log.Printf("LightRAG: healthy")
		} else {
			log.Printf("LightRAG: HTTP %d", resp.StatusCode)
		}
	}

	log.Printf("coderag: running (pid %d)", os.Getpid())
	return nil
}

func stopCmd(c *cli.Context) error {
	composeFile := findComposeFile()
	cmd := exec.Command("docker", "compose", "-f", composeFile, "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down failed: %w", err)
	}
	log.Print("All services stopped")
	return nil
}

func queryCmd(c *cli.Context) error {
	queryText := c.Args().First()
	if queryText == "" {
		return fmt.Errorf("usage: coderag query \"<text>\"")
	}

	client := crud.NewConfig()
	result, err := client.Query(queryText)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	fmt.Printf("%v\n", result)
	return nil
}

func mcpCmd(c *cli.Context) error {
	inputDir := inputDirOrDefault(c)
	if _, err := os.Stat(inputDir); err != nil {
		return fmt.Errorf("directory not found: %s", inputDir)
	}

	ws := projectName(inputDir)
	ctx := context.Background()
	if err := ensureMCP(ctx, ws); err != nil {
		return fmt.Errorf("failed to ensure MCP services: %w", err)
	}

	go startWatchdogBackground(inputDir)

	log.Print("Connecting to MCP SSE bridge...")
	return runSSEBridge(ctx)
}

func startWatchdogBackground(inputDir string) {
	cfg := &watcher.Config{
		InputDir:     inputDir,
		DebounceSecs: 0.5,
		MaxWorkers:   4,
		UseGit:       true,
		LightRAGConfig: crud.NewConfig(),
	}

	w, err := watcher.NewWatcher(cfg)
	if err != nil {
		log.Printf("Watchdog: failed to create watcher: %v", err)
		return
	}

	log.Printf("Watchdog: started, watching %s", inputDir)

	if err := w.Start(); err != nil {
		log.Printf("Watchdog: stopped with error: %v", err)
	} else {
		log.Printf("Watchdog: stopped")
	}
}

func projectName(inputDir string) string {
	name := filepath.Base(inputDir)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' {
			return '_'
		}
		return r
	}, name)
	return name
}

func captureDockerLogs(logsDir string) error {
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("create logs dir: %w", err)
	}
	composeFile := findComposeFile()
	cmd := exec.Command("docker", "compose", "-f", composeFile, "logs", "--no-color", "--timestamps")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("docker logs: %w", err)
	}
	logFile := filepath.Join(logsDir, fmt.Sprintf("docker_%s.log", time.Now().Format("20060102_150405")))
	if err := os.WriteFile(logFile, out, 0644); err != nil {
		return fmt.Errorf("write log file: %w", err)
	}
	log.Printf("Docker logs saved to %s", logFile)
	return nil
}

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

	log.Printf("Setting workspace to '%s' for project %s", ws, inputDir)
	if err := client.SetWorkspace(ws); err != nil {
		log.Printf("Warning: failed to set workspace: %v (continuing with default)", err)
	}

	log.Printf("Scanning %s for C# solution files...", inputDir)

	sln := findSolutionFile(inputDir)
	if sln == "" {
		return fmt.Errorf("no .sln file found in %s", inputDir)
	}
	log.Printf("Found solution: %s", sln)

	log.Printf("Running C# analyser...")
	results, err := pipeline.RunCSharpAnalyser(sln, analyserOut)
	if err != nil {
		return fmt.Errorf("C# analyser failed: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("C# analyser produced no output for %s", sln)
	}
	log.Printf("Analyser produced %d output files", len(results))

	log.Printf("Applying results to knowledge graph...")
	if err := applyScanResults(client, results); err != nil {
		return fmt.Errorf("failed to apply scan results: %w", err)
	}

	stats, err := client.GetAllEntities()
	if err != nil {
		log.Printf("Scan complete (could not get entity count: %v)", err)
	} else {
		log.Printf("Scan complete. Knowledge graph now contains %d entities", len(stats))
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

func applyScanResults(client *crud.Config, results []pipeline.AnalyserResult) error {
	parserResults := make([]struct{ Filename string; Content string }, len(results))
	for i, r := range results {
		parserResults[i] = struct{ Filename string; Content string }{r.Filename, r.Content}
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
			_, err := client.CreateEntity(
				action.EntityName,
				filter.FilterDescription(getString(data, "description")),
				getString(data, "entity_type", "UNKNOWN"),
				getString(data, "source_id", ""),
				getString(data, "file_path", ""),
			)
			if err == nil {
				createdEntities++
			} else {
				log.Printf("ERROR creating entity %q (type=%s, file=%s): %v", action.EntityName, getString(data, "entity_type", "UNKNOWN"), getString(data, "file_path", ""), err)
			}
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

	log.Printf("Entities: created=%d, updated=%d, deleted=%d", createdEntities, updatedEntities, deletedEntities)

	oldRelMap := loadCurrentRelationsScan(client)
	newRelMap := parserRelationsToMapScan(relations)
	relActions := diff.DiffRelations(oldRelMap, newRelMap)

	createdRels := 0
	deletedRels := 0

	for _, action := range relActions {
		switch action.Kind {
		case diff.ActionDelete:
			if _, err := client.DeleteRelation(action.Source, action.Target); err == nil {
				deletedRels++
			} else {
				log.Printf("ERROR deleting relation %q → %q: %v", action.Source, action.Target, err)
			}
		case diff.ActionCreate:
			data := filter.FilterRelationData(action.Data)
			_, err := client.CreateRelation(
				action.Source,
				action.Target,
				filter.FilterDescription(getString(data, "description")),
				getString(data, "keywords"),
				getFloat(data, "weight", 1.0),
				getString(data, "source_id", ""),
				getString(data, "file_path", ""),
			)
			if err == nil {
				createdRels++
			} else {
				log.Printf("ERROR creating relation %q → %q (keywords=%s, weight=%.1f, file=%s): %v", action.Source, action.Target, getString(data, "keywords"), getFloat(data, "weight", 1.0), getString(data, "file_path", ""), err)
			}
		case diff.ActionUpdate:
			data := filter.FilterRelationData(action.Data)
			if _, err := client.UpdateRelation(action.Source, action.Target, data); err == nil {
				createdRels++
			} else {
				log.Printf("ERROR updating relation %q → %q: %v", action.Source, action.Target, err)
			}
		}
	}

	log.Printf("Relations: created=%d, deleted=%d", createdRels, deletedRels)

	return nil
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

func parserRelationsToMapScan(relations []parser.Relation) map[string]map[string]any {
	result := make(map[string]map[string]any)
	for _, r := range relations {
		key := diff.RelationKey(r.Source, r.Target, r.Keywords)
		result[key] = map[string]any{
			"description": r.Description,
			"keywords":    r.Keywords,
			"weight":      r.Weight,
		}
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

		key := diff.RelationKey(src, tgt, kw)
		result[key] = edge
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

func runSSEBridge(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:9622/sse", nil)
	if err != nil {
		return fmt.Errorf("SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connect: %w", err)
	}
	defer resp.Body.Close()

	bodyReader := bufio.NewReader(resp.Body)

	endpointLine, err := bodyReader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read SSE endpoint line: %w", err)
	}
	endpointLine = strings.TrimSpace(endpointLine)

	if !strings.HasPrefix(endpointLine, "event: endpoint") {
		return fmt.Errorf("expected SSE event: endpoint, got: %s", endpointLine)
	}

	dataLine, err := bodyReader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read SSE data line: %w", err)
	}
	dataLine = strings.TrimSpace(dataLine)
	if !strings.HasPrefix(dataLine, "data: ") {
		return fmt.Errorf("expected SSE data line, got: %s", dataLine)
	}

	messageURL := strings.TrimPrefix(dataLine, "data: ")
	sessionID := extractSessionID(messageURL)
	if sessionID == "" {
		return fmt.Errorf("could not extract session_id from: %s", messageURL)
	}

	log.Printf("SSE session established: %s", sessionID)

	errCh := make(chan error, 2)

	go func() {
		stdin := bufio.NewReader(os.Stdin)
		for {
			line, err := stdin.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					errCh <- fmt.Errorf("stdin read: %w", err)
				}
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			postURL := fmt.Sprintf("http://localhost:9622/messages/?session_id=%s", sessionID)
			postReq, err := http.NewRequestWithContext(ctx, "POST", postURL, strings.NewReader(line))
			if err != nil {
				errCh <- fmt.Errorf("POST request: %w", err)
				return
			}
			postReq.Header.Set("Content-Type", "application/json")

			postResp, err := http.DefaultClient.Do(postReq)
			if err != nil {
				errCh <- fmt.Errorf("POST: %w", err)
				return
			}
			postResp.Body.Close()
		}
	}()

	go func() {
		for {
			line, err := bodyReader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					errCh <- fmt.Errorf("SSE read: %w", err)
				}
				return
			}
			line = strings.TrimSpace(line)

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				fmt.Println(data)
				os.Stdout.Sync()
			}
		}
	}()

	return <-errCh
}

func extractSessionID(messageURL string) string {
	idx := strings.Index(messageURL, "session_id=")
	if idx < 0 {
		return ""
	}
	val := messageURL[idx+len("session_id="):]
	if amp := strings.Index(val, "&"); amp >= 0 {
		val = val[:amp]
	}
	return val
}
