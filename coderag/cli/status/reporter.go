// Package status pushes coderag CLI progress events to the LightRAG API
// (/coderag/status/*) so the WebUI Scan tab can show live scan/watch activity.
//
// Fire-and-forget by design: emitting never blocks the pipeline, network
// failures are swallowed, and everything degrades to a no-op when disabled
// (CODERAG_STATUS_DISABLE=1) or unreachable.
package status

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// flushInterval controls how often queued events are batched into one POST.
// Package-level var so tests can shrink it.
var flushInterval = 400 * time.Millisecond

// maxQueue bounds the in-memory event backlog; excess events are dropped.
const maxQueue = 512

// ClientTimeout keeps a dead API from stalling the flusher.
const ClientTimeout = 2 * time.Second

// Event mirrors the Python StatusEvent model in
// lightrag/api/routers/coderag_status_routes.py — keep both in sync.
type Event struct {
	RunID   string         `json:"run_id"`
	Stage   string         `json:"stage"`
	Level   string         `json:"level,omitempty"`
	Message string         `json:"message,omitempty"`
	Detail  string         `json:"detail,omitempty"`
	State   string         `json:"state,omitempty"`
	TS      float64        `json:"ts,omitempty"`
	Counts  map[string]int `json:"counts,omitempty"`
}

// Reporter batches events and posts them asynchronously.
type Reporter struct {
	runID    string
	endpoint string
	client   *http.Client
	ch       chan Event
	wg       sync.WaitGroup
	enabled  bool
}

// Default is the process-wide reporter used by pipeline/watcher hooks.
// It may be nil; all methods are nil-safe no-ops in that case.
var Default *Reporter

// New creates a reporter posting to <baseURL>/coderag/status/events.
// Returns a disabled no-op reporter when baseURL is empty or
// CODERAG_STATUS_DISABLE is set.
func New(baseURL string) *Reporter {
	r := &Reporter{
		runID:    newRunID(),
		client:   &http.Client{Timeout: ClientTimeout},
		ch:       make(chan Event, maxQueue),
		enabled:  false,
		endpoint: strings.TrimRight(baseURL, "/") + "/coderag/status/events",
	}

	if baseURL == "" || isDisabled() {
		return r
	}

	r.enabled = true
	r.wg.Add(1)
	go r.loop()
	return r
}

func isDisabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("CODERAG_STATUS_DISABLE")))
	return v == "1" || v == "true" || v == "yes"
}

func newRunID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// RunID returns the id grouping all events of this reporter's run.
func (r *Reporter) RunID() string {
	if r == nil {
		return ""
	}
	return r.runID
}

// Stage emits an info-level progress event.
func (r *Reporter) Stage(stage, format string, args ...any) {
	r.emit(Event{Stage: stage, Level: "info", Message: fmt.Sprintf(format, args...)})
}

// Detail emits an info event with an explicit focus object (e.g. current file).
func (r *Reporter) Detail(stage, detail, format string, args ...any) {
	r.emit(Event{Stage: stage, Level: "info", Message: fmt.Sprintf(format, args...), Detail: detail})
}

// Warn emits a warning event.
func (r *Reporter) Warn(stage, format string, args ...any) {
	r.emit(Event{Stage: stage, Level: "warn", Message: fmt.Sprintf(format, args...)})
}

// Error emits an error event and flags the run as errored in the UI.
func (r *Reporter) Error(stage, format string, args ...any) {
	r.emit(Event{Stage: stage, Level: "error", State: "error", Message: fmt.Sprintf(format, args...)})
}

// Counts merges cumulative counters into the run snapshot.
func (r *Reporter) Counts(stage string, counts map[string]int) {
	r.emit(Event{Stage: stage, Level: "info", Counts: counts})
}

// Done marks the run finished successfully.
func (r *Reporter) Done(format string, args ...any) {
	r.emit(Event{Stage: "scan", Level: "info", State: "idle", Message: fmt.Sprintf(format, args...)})
}

// Close flushes remaining events and stops the flusher. Safe on nil/disabled.
func (r *Reporter) Close() {
	if r == nil || !r.enabled {
		return
	}
	close(r.ch)
	r.wg.Wait()
}

// SetDefault installs r as the process-wide reporter and returns a restore
// func (handy in tests).
func SetDefault(r *Reporter) func() {
	prev := Default
	Default = r
	return func() { Default = prev }
}

func (r *Reporter) emit(ev Event) {
	if r == nil || !r.enabled {
		return
	}
	ev.RunID = r.runID
	if ev.TS == 0 {
		ev.TS = float64(time.Now().UnixNano()) / 1e9
	}
	select {
	case r.ch <- ev:
	default: // queue full -> drop instead of blocking the pipeline
	}
}

// loop drains the queue in batches until Close.
func (r *Reporter) loop() {
	defer r.wg.Done()
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]Event, 0, 64)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		r.post(batch)
		batch = batch[:0]
	}

	for {
		select {
		case ev, ok := <-r.ch:
			if !ok {
				flush()
				return
			}
			batch = append(batch, ev)
			if len(batch) >= 64 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (r *Reporter) post(batch []Event) {
	body, err := json.Marshal(batch)
	if err != nil {
		return
	}
	resp, err := r.client.Post(r.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return // unreachable API -> drop silently, retry on next flush
	}
	resp.Body.Close()
}
