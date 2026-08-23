package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedComposeMatchesRepoFile(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", "..", "docker-compose.yml"))
	data, err := os.ReadFile(repo)
	if err != nil {
		t.Skipf("repo docker-compose.yml not found at %s", repo)
	}
	if string(embeddedCompose) != string(data) {
		t.Fatalf("embedded docker-compose.yml is out of sync with %s; update coderag/cli/docker-compose.yml", repo)
	}
}

func TestFindComposeFile(t *testing.T) {
	if f := findComposeFile(); f == "" {
		t.Fatal("findComposeFile returned empty; expected embedded fallback to materialize a file")
	}
}
