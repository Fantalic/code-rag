package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Container-engine selection. The CLI drives the LightRAG/MCP stack through
// `docker compose` or `podman compose`; which one is used is resolved once per
// run: CODERAG_ENGINE env var > CODERAG_ENGINE in <repoRoot>/.env > auto-detect
// (docker first, then podman).

// loadRepoEnvFile reads KEY=VALUE lines from the repo-root .env and exports
// every variable that is NOT already present in the process environment.
// Real environment variables always win over .env values.
func loadRepoEnvFile() {
	root := repoRoot()
	if root == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		// real environment variables always win over .env values
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		os.Setenv(key, val)
	}
}

// resolveEngine returns the container engine binary name ("docker"|"podman").
// Falls back to "docker" when nothing is detectable; callers surface a clear
// error when the binary cannot be executed.
func resolveEngine() string {
	loadRepoEnvFile()

	switch e := strings.ToLower(strings.TrimSpace(os.Getenv("CODERAG_ENGINE"))); e {
	case "podman":
		return "podman"
	case "docker", "":
	default:
		fmt.Fprintf(os.Stderr, "warning: unknown CODERAG_ENGINE=%q, falling back to auto-detect\n", e)
	}

	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	return "docker"
}

// composeCmd builds a `<engine> compose ...` command so start/stop/logs work
// with both Docker and Podman (podman >= 4.7 ships a compose provider).
func composeCmd(ctx context.Context, args ...string) *exec.Cmd {
	full := append([]string{"compose"}, args...)
	cmd := exec.CommandContext(ctx, resolveEngine(), full...)
	cmd.Env = os.Environ()
	return cmd
}
