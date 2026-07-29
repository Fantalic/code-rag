package lightrag

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// inMemoryTransport is an in-memory RoundTripper that invokes a handler directly
// without touching the network stack (required on machines that block loopback TCP).
type inMemoryTransport struct {
	handler http.Handler
}

func (t *inMemoryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	t.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

// testClient creates a Client backed by the given handler using an in-memory transport.
func testClient(handler http.HandlerFunc) *Client {
	transport := &inMemoryTransport{handler: handler}
	return &Client{
		BaseURL:    "http://test.local",
		Timeout:    5 * time.Second,
		HTTPClient: &http.Client{Transport: transport},
	}
}

func TestNew(t *testing.T) {
	t.Setenv("LIGHTRAG_URL", "")
	t.Setenv("LIGHTRAG_API_KEY", "")
	t.Setenv("LIGHTRAG_TIMEOUT", "")

	c := New()
	if c.BaseURL != "http://localhost:9621" {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL, "http://localhost:9621")
	}
	if c.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", c.APIKey)
	}
	if c.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", c.Timeout, DefaultTimeout)
	}
}

func TestHeaders(t *testing.T) {
	c := &Client{BaseURL: "http://localhost:9621"}
	h := c.Headers()
	if v, ok := h["Content-Type"]; !ok || v != "application/json" {
		t.Errorf("Content-Type = %q, want %q", v, "application/json")
	}
	if _, ok := h["X-API-Key"]; ok {
		t.Error("X-API-Key should not be set when APIKey is empty")
	}

	c2 := &Client{BaseURL: "http://localhost:9621", APIKey: "mykey"}
	h2 := c2.Headers()
	if v, ok := h2["X-API-Key"]; !ok || v != "mykey" {
		t.Errorf("X-API-Key = %q, want %q", v, "mykey")
	}
}

func TestCheckHealth(t *testing.T) {
	t.Run("200 returns true", func(t *testing.T) {
		c := testClient(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		})
		if !c.CheckHealth() {
			t.Error("CheckHealth() = false, want true")
		}
	})

	t.Run("500 returns false", func(t *testing.T) {
		c := testClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		if c.CheckHealth() {
			t.Error("CheckHealth() = true, want false")
		}
	})
}

func TestDo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c := testClient(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		})
		result, err := c.do("POST", "/test", map[string]string{"key": "val"})
		if err != nil {
			t.Fatalf("do() error = %v", err)
		}
		if result["status"] != "ok" {
			t.Errorf("result[\"status\"] = %v, want %q", result["status"], "ok")
		}
	})

	t.Run("HTTP error", func(t *testing.T) {
		c := testClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
		})
		_, err := c.do("GET", "/fail", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		c := testClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`not json`))
		})
		_, err := c.do("GET", "/badjson", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestDoRaw(t *testing.T) {
	c := testClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`raw data`))
	})
	data, err := c.doRaw("GET", "/raw", nil)
	if err != nil {
		t.Fatalf("doRaw() error = %v", err)
	}
	if string(data) != "raw data" {
		t.Errorf("body = %q, want %q", string(data), "raw data")
	}
}
