package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/coderag/crud"
	"github.com/coderag/graph"
	"github.com/coderag/status"
	"github.com/coderag/watcher"
	"github.com/urfave/cli/v2"
)

var Version = "dev"
var quietMode = false

const agentHelpText = `codeRAG - agent quick reference (full docs: docs/USAGE.md)

INITIALIZE / INDEX A PROJECT FOLDER (there is no init command):
  coderag scan <dir>           auto-detect *.sln / build.zig / Python, index changes
  coderag scan <dir> --clean   wipe graph + analyser output, then re-index
  coderag start <dir>          Docker services + live file watcher

QUERY / EXPLORE FROM THE CLI:
  coderag graph files|deps|cycles    derived file-level graph views
  coderag query "<text>"            LLM answer (needs Ollama)
  coderag status                    service + workspace health
  coderag projects list|focus|forget  host project registry

MCP TOOLS (preferred for agents): get_nodes, node_details, file_dependencies,
dead_code, detect_circular_dependencies, codebase_overview, get_source,
query_codebase, index_coverage, what_changed. All list tools accept
page/per_page (default 100). Each tool description refers back here.

HELP: coderag --help ; coderag help <command>
DOCS: docs/USAGE.md (CLI), docs/SUPPORTED_FILES.md (languages), docs/INSTALL.md (setup).`

func main() {
	app := &cli.App{
		Name:    "coderag",
		Usage:   "codeRAG — knowledge graph for codebases",
		Version: Version,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "quiet",
				Usage: "Suppress informational output, show only errors and results",
			},
			&cli.BoolFlag{
				Name:  "agent-help",
				Usage: "Print a compact agent guide, then exit",
			},
		},
		Before: func(c *cli.Context) error {
			quietMode = c.Bool("quiet")
			if c.Bool("agent-help") {
				fmt.Print(agentHelpText)
				return cli.Exit("", 0)
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:   "start",
				Usage:  "Start Docker services + file watcher",
				Action: startCmd,
				Args:   false,
			},
			{
				Name:   "mcp",
				Usage:  "Start MCP stdio↔SSE bridge for a project directory",
				Action: mcpCmd,
				Args:   true,
			},
			{
				Name:   "scan",
				Usage:  "Index a project (*.sln, build.zig, or Python) into the knowledge graph",
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
			{
				Name:  "graph",
				Usage: "Derived file-level views of the knowledge graph",
				Subcommands: []*cli.Command{
					{
						Name:   "files",
						Usage:  "Per-file stats table (nodes, fan-in, fan-out)",
						Action: graphFilesCmd,
						Flags:  graphFlags(),
					},
					{
						Name:   "deps",
						Usage:  "Show file dependencies (refs + dependents)",
						Action: graphDepsCmd,
						Args:   true,
						Flags:  graphFlags(),
					},
					{
						Name:   "cycles",
						Usage:  "Detect dependency cycles between files",
						Action: graphCyclesCmd,
						Flags:  graphFlags(),
					},
				},
			},
			{
				Name:  "projects",
				Usage: "Manage the host-CLI project registry (list, focus, forget)",
				Subcommands: []*cli.Command{
					{
						Name:   "list",
						Usage:  "List every project the CLI has ever scanned or watched",
						Action: projectsListCmd,
					},
					{
						Name:   "focus",
						Usage:  "Set the focused project (path or id)",
						Action: projectsFocusCmd,
						Args:   true,
					},
					{
						Name:   "forget",
						Usage:  "Remove a project from the registry",
						Action: projectsForgetCmd,
						Args:   true,
					},
				},
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

	if !quietMode {
		log.Print("MCP not reachable at :9622, ensuring Docker services are up...")
	}
	if err := ensureLightRAG(ctx, workspace); err != nil {
		return err
	}

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Waiting for MCP SSE to become healthy..."
	s.Writer = os.Stderr
	if !quietMode {
		s.Start()
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-timeout:
			if !quietMode {
				s.Stop()
			}
			return fmt.Errorf("timeout waiting for MCP at http://localhost:9622/health")
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:9622/health", nil)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 400 {
					if !quietMode {
						s.Stop()
						log.Print("MCP SSE is healthy")
					}
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
					if !quietMode {
						log.Printf("LightRAG is healthy (workspace: '%s')", health.Configuration.Workspace)
					}
					return nil
				}
				if !quietMode {
					log.Printf("Workspace mismatch: container has '%s', need '%s'", health.Configuration.Workspace, workspace)
				}
			} else {
				resp.Body.Close()
			}
		} else if resp != nil {
			resp.Body.Close()
		}
	}

	// Need to restart with the correct workspace
	composeFile := findComposeFile()
	if composeFile == "" {
		return fmt.Errorf("docker-compose.yml not found; run coderag from the codeRAG repo root or set CODERAG_COMPOSE=<path>")
	}

	if !quietMode {
		log.Printf("Starting Docker services with workspace '%s'...", workspace)
	}

	// Stop existing containers first (silently)
	downCmd := composeCmd(ctx, "-f", composeFile, "down")
	downCmd.Stdout = nil
	downCmd.Stderr = nil
	_ = downCmd.Run()

	// Start with the correct workspace (silently)
	upCmd := composeCmd(ctx, "-f", composeFile, "up", "-d")
	upCmd.Env = append(os.Environ(), "WORKSPACE="+workspace)
	if err := upCmd.Run(); err != nil {
		return fmt.Errorf("docker compose failed: %w", err)
	}

	// Spinner while waiting for health
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Waiting for LightRAG to become healthy..."
	s.Writer = os.Stderr
	if !quietMode {
		s.Start()
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	timeout := time.After(120 * time.Second)

	for {
		select {
		case <-timeout:
			if !quietMode {
				s.Stop()
			}
			return fmt.Errorf("timeout waiting for LightRAG at http://localhost:9621/health")
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:9621/health", nil)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 400 {
					if !quietMode {
						s.Stop()
						log.Print("LightRAG is healthy")
					}
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
	if p := os.Getenv("CODERAG_COMPOSE"); p != "" {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}

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

	if f := find("docker-compose.yml"); f != "" {
		return f
	}
	if f := find("../docker-compose.yml"); f != "" {
		return f
	}
	if exe, err := os.Executable(); err == nil {
		if f := find(filepath.Join(filepath.Dir(exe), "docker-compose.yml")); f != "" {
			return f
		}
	}

	walkUp := func(start string) string {
		for d := start; ; d = filepath.Dir(d) {
			if f := find(filepath.Join(d, "docker-compose.yml")); f != "" {
				return f
			}
			if filepath.Dir(d) == d {
				break
			}
		}
		return ""
	}
	if wd, err := os.Getwd(); err == nil {
		if f := walkUp(wd); f != "" {
			return f
		}
	}
	if exe, err := os.Executable(); err == nil {
		if f := walkUp(filepath.Dir(exe)); f != "" {
			return f
		}
	}

	return materializeComposeFile()
}

// materializeComposeFile writes the embedded docker-compose.yml into a
// per-user cache directory so coderag works from any working directory.
func materializeComposeFile() string {
	dir := userCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	content := string(embeddedCompose)
	if repo := repoRoot(); repo != "" {
		content = strings.ReplaceAll(content, "context: .", "context: "+repo)
		content = strings.ReplaceAll(content, "./rag_input", filepath.Join(repo, "rag_input"))
	}
	dst := filepath.Join(dir, "docker-compose.yml")
	if cur, err := os.ReadFile(dst); err == nil && string(cur) == content {
		syncRepoEnvFile(dir)
		return dst
	}
	if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
		return ""
	}
	syncRepoEnvFile(dir)
	return dst
}

// syncRepoEnvFile copies the repo-root .env next to the cached compose file so
// ${VAR} substitution in docker-compose.yml keeps working when coderag runs
// from any folder (compose reads .env from the project directory).
func syncRepoEnvFile(composeDir string) {
	root := repoRoot()
	if root == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		return // no repo .env; nothing to sync
	}
	dst := filepath.Join(composeDir, ".env")
	if cur, err := os.ReadFile(dst); err == nil && string(cur) == string(data) {
		return
	}
	_ = os.WriteFile(dst, data, 0o644)
}

func userCacheDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "coderag")
	}
	return filepath.Join(os.TempDir(), "coderag")
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
	if !quietMode {
		log.Printf("Setting workspace to '%s' for project %s", ws, inputDir)
	}
	if err := client.SetWorkspace(ws); err != nil {
		log.Printf("Warning: failed to set workspace: %v (continuing with default)", err)
	}

	// Status reporter for the whole watch session (WebUI Scan tab live view)
	rep := status.New(client.BaseURL)
	defer rep.Close()
	restoreRep := status.SetDefault(rep)
	defer restoreRep()
	rep.Stage("watch", "watch session started: %s", inputDir)

	cfg := &watcher.Config{
		InputDir:       inputDir,
		DebounceSecs:   0.5,
		MaxWorkers:     4,
		UseGit:         true,
		LightRAGConfig: crud.NewConfig(),
	}

	w, err := watcher.NewWatcher(cfg)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	if err := w.Start(); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	if !quietMode {
		log.Printf("Watching %s (press Ctrl+C to stop)", inputDir)
		log.Printf("WebUI: http://localhost:9621")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	if !quietMode {
		log.Print("Shutting down...")
	}

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
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
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

	// Status reporter for the whole watch session (WebUI Scan tab live view)
	rep := status.New(lightragURL)
	defer rep.Close()
	restoreRep := status.SetDefault(rep)
	defer restoreRep()
	rep.Stage("watch", "watch session started: %s", inputDir)
	wsName := projectName(inputDir)
	pushProjectStart(inputDir, wsName, "csharp", lightragURL)

	w, err := watcher.NewWatcher(cfg)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	if err := w.Start(); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	if !quietMode {
		log.Printf("Watching %s (press Ctrl+C to stop)", inputDir)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	if !quietMode {
		log.Print("Shutting down...")
	}

	w.Stop()
	return nil
}

func statusCmd(c *cli.Context) error {
	client := crud.NewConfig()
	lightragStatus := "not running"
	workspace := "-"

	req, err := http.NewRequest("GET", client.BaseURL+"/health", nil)
	if err == nil {
		for k, v := range client.Headers() {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode < 400 {
				var health struct {
					Configuration struct {
						Workspace string `json:"workspace"`
					} `json:"configuration"`
				}
				if json.NewDecoder(resp.Body).Decode(&health) == nil {
					workspace = health.Configuration.Workspace
				}
				lightragStatus = "running"
			} else {
				lightragStatus = fmt.Sprintf("error (HTTP %d)", resp.StatusCode)
			}
		}
	}

	mcpStatus := "not running"
	mcpResp, err := http.Get("http://localhost:9622/health")
	if err == nil {
		mcpResp.Body.Close()
		if mcpResp.StatusCode < 400 {
			mcpStatus = "running"
		}
	}

	fmt.Printf("\n  codeRAG v%s\n\n", Version)
	fmt.Printf("  %-20s %s\n", "LightRAG API:", lightragStatus)
	fmt.Printf("  %-20s %s\n", "MCP SSE:", mcpStatus)
	fmt.Printf("  %-20s %s\n", "Workspace:", workspace)
	fmt.Printf("  %-20s http://localhost:9621\n", "WebUI:")

	return nil
}

func stopCmd(c *cli.Context) error {
	composeFile := findComposeFile()
	if composeFile == "" {
		return fmt.Errorf("docker-compose.yml not found; run coderag from the codeRAG repo root or set CODERAG_COMPOSE=<path>")
	}
	cmd := composeCmd(context.Background(), "-f", composeFile, "down")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down failed: %w", err)
	}
	if !quietMode {
		log.Print("All services stopped")
	}
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

func graphFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "workspace",
			Aliases: []string{"w"},
			Usage:   "LightRAG workspace to query (default: currently active)",
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "url",
			Aliases: []string{"u"},
			Usage:   "LightRAG API URL",
			EnvVars: []string{"LIGHTRAG_URL"},
			Value:   "http://localhost:9621",
		},
	}
}

func loadFileGraph(c *cli.Context) (*graph.FileGraph, error) {
	client := &crud.Config{BaseURL: c.String("url")}
	if ws := c.String("workspace"); ws != "" {
		if err := client.SetWorkspace(ws); err != nil {
			return nil, fmt.Errorf("set workspace %q: %w", ws, err)
		}
	}
	snap, err := client.GetGraphSnapshot()
	if err != nil {
		return nil, fmt.Errorf("fetch graph: %w", err)
	}
	return graph.Build(snap.Nodes, snap.Edges), nil
}

func graphFilesCmd(c *cli.Context) error {
	g, err := loadFileGraph(c)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "FILE\tNODES\tFAN-IN\tFAN-OUT")
	for _, s := range g.Stats() {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\n", s.FilePath, s.Nodes, s.FanIn, s.FanOut)
	}
	w.Flush()
	return nil
}

func graphDepsCmd(c *cli.Context) error {
	arg := c.Args().First()
	if arg == "" {
		return fmt.Errorf("usage: coderag graph deps <file>")
	}

	g, err := loadFileGraph(c)
	if err != nil {
		return err
	}

	file, ok := g.ResolveFile(arg)
	if !ok {
		return fmt.Errorf("no file in the graph matches %q", arg)
	}

	fmt.Printf("File: %s\n", file)

	refs := g.Refs(file)
	if len(refs) == 0 {
		fmt.Println("References: (none)")
	} else {
		fmt.Printf("References (%d):\n", len(refs))
		for _, r := range refs {
			fmt.Printf("  %s\n", r)
		}
	}

	deps := g.Dependents(file)
	if len(deps) == 0 {
		fmt.Println("Dependents: (none)")
	} else {
		fmt.Printf("Dependents (%d):\n", len(deps))
		for _, d := range deps {
			fmt.Printf("  %s\n", d)
		}
	}

	entities := g.Entities(file)
	if len(entities) > 0 {
		fmt.Printf("Entities (%d):\n", len(entities))
		for _, e := range entities {
			fmt.Printf("  %s\n", e)
		}
	}
	return nil
}

func graphCyclesCmd(c *cli.Context) error {
	g, err := loadFileGraph(c)
	if err != nil {
		return err
	}

	cycles := g.Cycles()
	if len(cycles) == 0 {
		fmt.Println("No dependency cycles found.")
		return nil
	}
	fmt.Printf("Dependency cycles found (%d):\n", len(cycles))
	for i, cyc := range cycles {
		fmt.Printf("  Cycle %d: %s\n", i+1, strings.Join(cyc, " -> "))
	}
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
		InputDir:       inputDir,
		DebounceSecs:   0.5,
		MaxWorkers:     4,
		UseGit:         true,
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
	if composeFile == "" {
		return fmt.Errorf("docker-compose.yml not found; run coderag from the codeRAG repo root or set CODERAG_COMPOSE=<path>")
	}
	cmd := composeCmd(context.Background(), "-f", composeFile, "logs", "--no-color", "--timestamps")
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
