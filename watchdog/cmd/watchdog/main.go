package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/code-lightrag/watchdog/internal/crud"
	"github.com/code-lightrag/watchdog/internal/watcher"
)

var lightragProcess *exec.Cmd
var mcpProcess *exec.Cmd

func main() {
	app := &cli.App{
		Name:  "watchdog",
		Usage: "CodeLightRAG file watcher — syncs codebase changes to knowledge graph",
		Commands: []*cli.Command{
			{
				Name:   "watch",
				Usage:  "Watch a project folder and sync changes to LightRAG",
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
					&cli.BoolFlag{
						Name:    "git",
						Aliases: []string{"g"},
						Usage:   "Use git to detect changes instead of filesystem watching",
						EnvVars: []string{"USE_GIT_WATCHER"},
					},
					&cli.StringFlag{
						Name:    "api-key",
						Aliases: []string{"k"},
						Usage:   "LightRAG API key",
						EnvVars: []string{"LIGHTRAG_API_KEY"},
					},
				},
			},
			{
				Name:   "start",
				Usage:  "Start LightRAG + MCP server + watchdog together",
				Action: startCmd,
				Args:   true,
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

func watchCmd(c *cli.Context) error {
	inputDir := c.Args().First()
	if inputDir == "" {
		return fmt.Errorf("usage: watchdog watch <directory>")
	}

	if _, err := os.Stat(inputDir); err != nil {
		return fmt.Errorf("directory not found: %s", inputDir)
	}

	os.Setenv("INPUT_DIR", inputDir)

	lightragURL := c.String("url")
	if lightragURL == "" {
		lightragURL = os.Getenv("LIGHTRAG_URL")
		if lightragURL == "" {
			lightragURL = "http://localhost:9621"
		}
	}

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
			APIKey:  c.String("api-key"),
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
	waitForSignal(sigCh)

	w.Stop()
	return nil
}

func startCmd(c *cli.Context) error {
	inputDir := c.Args().First()
	if inputDir == "" {
		return fmt.Errorf("usage: watchdog start <directory>")
	}

	os.Setenv("INPUT_DIR", inputDir)

	ollamaHost := c.String("ollama")
	if ollamaHost == "" {
		ollamaHost = os.Getenv("OLLAMA_HOST")
		if ollamaHost == "" {
			ollamaHost = "http://localhost:11434"
		}
	}

	log.Printf("Starting LightRAG API...")

	lightragPath, err := findLightRAGBinary()
	if err != nil {
		return err
	}

	env := append(os.Environ(),
		"OLLAMA_HOST="+ollamaHost,
		"LIGHTRAG_PORT=9621",
		"LIGHTRAG_WORKING_DIR="+filepath.Join(inputDir, ".coderag", "rag_storage"),
	)
	lightragProcess = exec.Command(lightragPath)
	lightragProcess.Stdout = os.Stdout
	lightragProcess.Stderr = os.Stderr
	lightragProcess.Env = env
	lightragProcess.Dir = inputDir

	if err := lightragProcess.Start(); err != nil {
		return fmt.Errorf("start lightrag: %w", err)
	}

	log.Printf("LightRAG started (PID %d), waiting for health...", lightragProcess.Process.Pid)
	if err := waitForLightRAG("http://localhost:9621/health", 60*time.Second); err != nil {
		return fmt.Errorf("lightrag not healthy: %w", err)
	}

	log.Printf("LightRAG healthy")

	mcpPath, err := findMCPBinary()
	if err != nil {
		log.Printf("MCP binary not found, skipping: %v", err)
	} else {
		mcpEnv := append(os.Environ(), "LIGHTRAG_URL=http://localhost:9621")
		mcpProcess = exec.Command(mcpPath)
		mcpProcess.Stdout = os.Stdout
		mcpProcess.Stderr = os.Stderr
		mcpProcess.Env = mcpEnv
		if err := mcpProcess.Start(); err != nil {
			log.Printf("Failed to start MCP: %v", err)
		} else {
			log.Printf("MCP server started (PID %d)", mcpProcess.Process.Pid)
		}
	}

	cfg := &watcher.Config{
		InputDir:     inputDir,
		DebounceSecs: 0.5,
		MaxWorkers:   4,
		UseGit:       true,
		LightRAGConfig: &crud.Config{
			BaseURL: "http://localhost:9621",
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
	waitForSignal(sigCh)

	log.Printf("Stopping services...")
	w.Stop()
	if lightragProcess != nil {
		lightragProcess.Process.Kill()
	}
	if mcpProcess != nil {
		mcpProcess.Process.Kill()
	}

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

	log.Printf("Watchdog: running (pid %d)", os.Getpid())
	return nil
}

func stopCmd(c *cli.Context) error {
	if lightragProcess != nil {
		lightragProcess.Process.Kill()
		log.Printf("LightRAG stopped")
	}
	if mcpProcess != nil {
		mcpProcess.Process.Kill()
		log.Printf("MCP server stopped")
	}
	log.Printf("All services stopped")
	return nil
}

func queryCmd(c *cli.Context) error {
	queryText := c.Args().First()
	if queryText == "" {
		return fmt.Errorf("usage: watchdog query \"<text>\"")
	}

	client := crud.NewConfig()
	result, err := client.Query(queryText)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	fmt.Printf("%v\n", result)
	return nil
}

func findLightRAGBinary() (string, error) {
	paths := []string{
		"LightRAG/lightrag-server.exe",
		"./LightRAG/lightrag-server.exe",
		"../LightRAG/lightrag-server.exe",
		os.Getenv("LIGHTRAG_BINARY"),
	}
	for _, p := range paths {
		if p != "" {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	if which, err := exec.LookPath("lightrag-server"); err == nil {
		return which, nil
	}
	return "", fmt.Errorf("LightRAG binary not found. Set LIGHTRAG_BINARY or ensure lightrag-server is in PATH")
}

func findMCPBinary() (string, error) {
	paths := []string{
		"daniel-lightrag-mcp/mcp-server.exe",
		"./daniel-lightrag-mcp/mcp-server.exe",
		"../daniel-lightrag-mcp/mcp-server.exe",
		os.Getenv("MCP_BINARY"),
	}
	for _, p := range paths {
		if p != "" {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("MCP binary not found")
}

func waitForLightRAG(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for LightRAG at %s", url)
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 400 {
					return nil
				}
			}
		}
	}
}

func waitForSignal(ch chan os.Signal) {
	for {
		time.Sleep(time.Second)
	}
}