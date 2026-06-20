package main

import (
	"bufio"
	"context"
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
	"github.com/code-lightrag/coderag/pkg/watchdog/crud"
	"github.com/code-lightrag/coderag/pkg/watchdog/watcher"
)

func main() {
	app := &cli.App{
		Name:  "coderag",
		Usage: "CodeLightRAG — knowledge graph for codebases",
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

func ensureMCP(ctx context.Context) error {
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
	if err := ensureLightRAG(ctx); err != nil {
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

func ensureLightRAG(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:9621/health", nil)
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

	log.Print("LightRAG not reachable, starting Docker services...")

	composeFile := findComposeFile()
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose failed: %w", err)
	}

	log.Print("Waiting for LightRAG to become healthy...")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.After(60 * time.Second)

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

func startCmd(c *cli.Context) error {
	inputDir := c.Args().First()
	if inputDir == "" {
		return fmt.Errorf("usage: coderag start <directory>")
	}
	if _, err := os.Stat(inputDir); err != nil {
		return fmt.Errorf("directory not found: %s", inputDir)
	}

	ctx := context.Background()
	if err := ensureLightRAG(ctx); err != nil {
		return fmt.Errorf("failed to start LightRAG: %w", err)
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
	inputDir := c.Args().First()
	if inputDir == "" {
		return fmt.Errorf("usage: coderag watch <directory>")
	}
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
	inputDir := c.Args().First()
	if inputDir == "" {
		return fmt.Errorf("usage: coderag mcp <directory>")
	}
	if _, err := os.Stat(inputDir); err != nil {
		return fmt.Errorf("directory not found: %s", inputDir)
	}

	ctx := context.Background()
	if err := ensureMCP(ctx); err != nil {
		return fmt.Errorf("failed to ensure MCP services: %w", err)
	}

	log.Print("Connecting to MCP SSE bridge...")
	return runSSEBridge(ctx)
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
