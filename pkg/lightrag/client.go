// Package lightrag provides a Go HTTP client for the LightRAG REST API.
// Used by both the MCP server (pkg/mcp) and the file watcher (pkg/watchdog).
package lightrag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const DefaultTimeout = 30 * time.Second

type Client struct {
	BaseURL    string
	APIKey     string
	Timeout    time.Duration
	HTTPClient *http.Client // optional, for testing
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: c.Timeout}
}

func New() *Client {
	url := os.Getenv("LIGHTRAG_URL")
	if url == "" {
		url = "http://localhost:9621"
	}
	timeout := os.Getenv("LIGHTRAG_TIMEOUT")
	d := DefaultTimeout
	if timeout != "" {
		if t, err := time.ParseDuration(timeout); err == nil {
			d = t
		}
	}
	return &Client{
		BaseURL: url,
		APIKey:  os.Getenv("LIGHTRAG_API_KEY"),
		Timeout: d,
	}
}

func (c *Client) Headers() map[string]string {
	h := map[string]string{"Content-Type": "application/json"}
	if c.APIKey != "" {
		h["X-API-Key"] = c.APIKey
	}
	return h
}

func (c *Client) do(method, path string, payload interface{}) (map[string]any, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		body = bytes.NewReader(b)
	}

	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	for k, v := range c.Headers() {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return result, nil
}

func (c *Client) doRaw(method, path string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		body = bytes.NewReader(b)
	}

	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	for k, v := range c.Headers() {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return data, nil
}

func (c *Client) CheckHealth() bool {
	req, err := http.NewRequest("GET", c.BaseURL+"/health", nil)
	if err != nil {
		return false
	}
	for k, v := range c.Headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400
}
