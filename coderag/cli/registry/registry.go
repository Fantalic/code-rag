// Package registry keeps the persistent host-side list of projects that
// coderag has ever scanned or watched, plus the currently focused project.
//
// Storage: a JSON file (default ~/.coderag/projects.json, overridable via
// CODERAG_REGISTRY_FILE) and a sibling current.json holding the focused id.
//
// The LightRAG server mirrors a copy via POST /coderag/projects; this
// package is the authoritative source on the host.
package registry

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Project represents a project the CLI knows about.
type Project struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Path          string         `json:"path"`
	Kind          string         `json:"kind,omitempty"` // "csharp", "mixed", etc.
	CreatedAt     int64          `json:"created_at"`
	LastScanAt    int64          `json:"last_scan_at,omitempty"`
	LastStatus    string         `json:"last_status,omitempty"` // idle|scanning|error
	LastMessage   string         `json:"last_message,omitempty"`
	TotalCounts   map[string]int `json:"total_counts,omitempty"`
	ErrorsTotal   int            `json:"errors_total,omitempty"`
	ScanCount     int            `json:"scan_count,omitempty"`
}

// Registry is a thread-safe persistent collection of Projects.
type Registry struct {
	mu       sync.RWMutex
	items    map[string]*Project // by id
	filePath string

	// path to current.json; "" if persistence is disabled.
	currentPath string
	currentID   string
}

// New opens (or creates) a registry file. Pass an empty path to disable
// I/O — useful for tests.
func New(filePath string) *Registry {
	r := &Registry{
		items:       make(map[string]*Project),
		filePath:    filePath,
		currentPath: currentFilePath(filePath),
	}
	if filePath == "" {
		return r
	}
	_ = r.load()
	return r
}

// DefaultPath returns CODERAG_REGISTRY_FILE or ~/.coderag/projects.json.
func DefaultPath() string {
	if p := strings.TrimSpace(os.Getenv("CODERAG_REGISTRY_FILE")); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".coderag", "projects.json")
}

// currentFilePath swaps projects.json -> current.json in the same dir.
func currentFilePath(p string) string {
	if p == "" {
		return ""
	}
	dir := filepath.Dir(p)
	base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	return filepath.Join(dir, base+".current.json")
}

// NewID derives a stable short id from a project path.
func NewID(path string) string {
	abs, _ := filepath.Abs(path)
	sum := sha1.Sum([]byte(strings.ToLower(filepath.ToSlash(abs))))
	return hex.EncodeToString(sum[:])[:12]
}

// List returns a snapshot of all projects sorted by most-recent scan.
func (r *Registry) List() []Project {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Project, 0, len(r.items))
	for _, p := range r.items {
		out = append(out, *p)
	}
	// sort: latest scan first, then created_at
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].LastScanAt > out[i].LastScanAt ||
				(out[j].LastScanAt == out[i].LastScanAt && out[j].CreatedAt > out[i].CreatedAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// Get returns a project by id (or "" if absent).
func (r *Registry) Get(id string) (Project, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.items[id]
	if !ok {
		return Project{}, false
	}
	return *p, true
}

// Upsert inserts or updates a project and persists to disk.
// deltaCounts (may be nil) is merged into TotalCounts.
func (r *Registry) Upsert(p Project, deltaCounts map[string]int) (Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p.ID == "" {
		p.ID = NewID(p.Path)
	}
	if p.Name == "" {
		p.Name = filepath.Base(p.Path)
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = time.Now().Unix()
	}

	existing, ok := r.items[p.ID]
	if ok {
		// Preserve historical totals across scans.
		if p.TotalCounts == nil {
			p.TotalCounts = existing.TotalCounts
		}
		p.CreatedAt = existing.CreatedAt
		p.ScanCount = existing.ScanCount
	}

	if deltaCounts != nil {
if p.TotalCounts == nil {
		p.TotalCounts = make(map[string]int, len(deltaCounts))
	}
	for k, v := range deltaCounts {
		p.TotalCounts[k] += v
	}

	// Every upsert with deltaCounts = one scan completion recorded.
	p.ScanCount++
	}

	r.items[p.ID] = &p

	out := p
	if r.filePath != "" {
		if err := r.saveLocked(); err != nil {
			return out, err
		}
	}
	return out, nil
}

// SetFocused sets the currently focused project id and writes current.json.
// Empty id clears the focus.
func (r *Registry) SetFocused(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id != "" {
		if _, ok := r.items[id]; !ok {
			return fmt.Errorf("project %q not found", id)
		}
	}
	r.currentID = id
	if r.currentPath == "" {
		return nil
	}
	return writeJSONAtomic(r.currentPath, map[string]any{"id": id, "ts": time.Now().Unix()})
}

// Focused returns the currently focused project (or false).
func (r *Registry) Focused() (Project, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.currentID == "" {
		return Project{}, false
	}
	p, ok := r.items[r.currentID]
	if !ok {
		return Project{}, false
	}
	return *p, true
}

// Forget removes a project by id.
func (r *Registry) Forget(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return fmt.Errorf("project %q not found", id)
	}
	delete(r.items, id)
	if r.currentID == id {
		r.currentID = ""
	}
	if r.filePath != "" {
		if err := r.saveLocked(); err != nil {
			return err
		}
	}
	if r.currentPath != "" {
		_ = writeJSONAtomic(r.currentPath, map[string]any{"id": "", "ts": time.Now().Unix()})
	}
	return nil
}

// load reads the registry file from disk.
func (r *Registry) load() error {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var raw struct {
		Projects []Project `json:"projects"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for i := range raw.Projects {
		p := raw.Projects[i]
		r.items[p.ID] = &p
	}
	if data, err := os.ReadFile(r.currentPath); err == nil {
		var cur struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(data, &cur)
		if _, ok := r.items[cur.ID]; ok {
			r.currentID = cur.ID
		}
	}
	return nil
}

// saveLocked writes to disk under the registry's write lock.
func (r *Registry) saveLocked() error {
	projects := make([]Project, 0, len(r.items))
	for _, p := range r.items {
		projects = append(projects, *p)
	}
	return writeJSONAtomic(r.filePath, map[string]any{"projects": projects})
}

// writeJSONAtomic writes via tmp-file + rename for crash safety.
func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
