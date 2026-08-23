// Package projects keeps the host-CLI's local registry of projects and
// mirrors a copy to the LightRAG server so the WebUI can list/focus them.
//
// Pushing is fire-and-forget: a failed POST does not bring down a scan
// (the local file is the source of truth). The server re-hydrates from
// this file on container startup.
package projects

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coderag/registry"
)

// Client posts project updates to the LightRAG server.
type Client struct {
	baseURL    string
	endpoint   string
	httpClient *http.Client
	enabled    bool

	mu      sync.Mutex
	closed  bool
}

// New returns a fire-and-forget client. baseURL="" or CODERAG_PROJECTS_DISABLE
// yields a disabled client (no I/O attempted).
func New(baseURL string) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 2 * time.Second},
	}
	if c.baseURL == "" || isDisabled() {
		return c
	}
	c.endpoint = c.baseURL + "/coderag/projects"
	c.enabled = true
	return c
}

func isDisabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("CODERAG_PROJECTS_DISABLE")))
	return v == "1" || v == "true" || v == "yes"
}

// Push upserts a project to the server. focus=true also marks it active.
// Errors are returned only for inspection; callers typically log-and-swallow.
func (c *Client) Push(p registry.Project, focus bool) error {
	if c == nil || !c.enabled {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("client closed")
	}
	payload := map[string]any{
		"project": p,
		"focus":   focus,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := os.Getenv("CODERAG_API_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("project push: HTTP %d", resp.StatusCode)
	}
	return nil
}

// FocusPOST sets the active project id on the server.
func (c *Client) FocusPOST(id string) error {
	if c == nil || !c.enabled {
		return nil
	}
	url := c.baseURL + "/coderag/projects/" + id + "/focus"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	if tok := os.Getenv("CODERAG_API_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("focus push: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Close stops further pushes.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
}
