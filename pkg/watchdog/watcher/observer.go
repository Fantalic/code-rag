package watcher

import (
	"crypto/md5"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/code-lightrag/coderag/pkg/watchdog/crud"
	"github.com/code-lightrag/coderag/pkg/watchdog/pipeline"
	"github.com/code-lightrag/coderag/pkg/watchdog/router"
)

type Config struct {
	InputDir       string
	DebounceSecs   float64
	MaxWorkers     int
	LightRAGConfig *crud.Config
	UseGit         bool
}

type Event struct {
	Path       string
	ChangeType string
	Results    []pipeline.AnalyserResult
}

type Debouncer struct {
	debounce time.Duration
	lock     sync.Mutex
	timers   map[string]*time.Timer
}

func NewDebouncer(seconds float64) *Debouncer {
	return &Debouncer{
		debounce: time.Duration(float64(time.Second) * seconds),
		timers:   make(map[string]*time.Timer),
	}
}

func (d *Debouncer) Schedule(path string, fn func()) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if t, ok := d.timers[path]; ok {
		t.Stop()
	}
	d.timers[path] = time.AfterFunc(d.debounce, func() {
		d.lock.Lock()
		delete(d.timers, path)
		d.lock.Unlock()
		fn()
	})
}

type Watcher struct {
	config       *Config
	watcher      *fsnotify.Watcher
	debouncer    *Debouncer
	workQueue    chan Event
	stopEvent    chan struct{}
	fileHashes   map[string]string
	hashLock     sync.RWMutex
	wg           sync.WaitGroup
	solutionLock sync.Mutex
	solutions    map[string]chan struct{}
}

func NewWatcher(cfg *Config) (*Watcher, error) {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 4
	}
	if cfg.DebounceSecs <= 0 {
		cfg.DebounceSecs = 0.5
	}
	if os.Getenv("USE_GIT_WATCHER") == "false" || os.Getenv("USE_GIT_WATCHER") == "0" {
		cfg.UseGit = false
	}
	return &Watcher{
		config:    cfg,
		debouncer: NewDebouncer(cfg.DebounceSecs),
		workQueue: make(chan Event, 1000),
		stopEvent: make(chan struct{}),
		fileHashes: make(map[string]string),
		solutions: make(map[string]chan struct{}),
	}, nil
}

func (w *Watcher) Start() error {
	log.Printf("Watcher: performing initial full scan of %s", w.config.InputDir)
	if err := w.initialSync(); err != nil {
		log.Printf("Initial sync warning: %v", err)
	}

	if w.config.UseGit {
		log.Printf("Watcher: using git-based change detection")
		go w.gitWatcher()
	} else {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return fmt.Errorf("new watcher: %w", err)
		}
		w.watcher = watcher

		if err := filepath.Walk(w.config.InputDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				return nil
			}
			return w.watcher.Add(path)
		}); err != nil {
			return fmt.Errorf("walk input dir: %w", err)
		}

		go w.readEvents()
	}

	log.Printf("Watcher: starting %d workers", w.config.MaxWorkers)
	for i := 0; i < w.config.MaxWorkers; i++ {
		w.wg.Add(1)
		go w.worker()
	}

	log.Printf("Watcher: monitoring %s", w.config.InputDir)
	return nil
}

func (w *Watcher) initialSync() error {
	files, err := w.collectSupportedFiles()
	if err != nil {
		return fmt.Errorf("collect files: %w", err)
	}

	log.Printf("Initial sync: found %d supported files", len(files))

	solutions := w.groupFilesBySolution(files)
	log.Printf("Initial sync: found %d solution(s)", len(solutions))

	for sln, slnFiles := range solutions {
		results, err := w.analyseSolution(sln)
		if err != nil {
			log.Printf("Initial sync: analyser failed for %s: %v", sln, err)
			continue
		}

		for _, filePath := range slnFiles {
			hash, err := fileHash(filePath)
			if err != nil {
				continue
			}
			w.hashLock.Lock()
			w.fileHashes[filePath] = hash
			w.hashLock.Unlock()

			w.workQueue <- Event{
				Path:       filePath,
				ChangeType: "initial",
				Results:    results,
			}
		}
	}

	log.Printf("Initial sync: enqueued all files for processing")
	return nil
}

func (w *Watcher) groupFilesBySolution(files []string) map[string][]string {
	solutions := make(map[string][]string)
	for _, filePath := range files {
		sln := findSolution(filePath)
		if sln == "" {
			continue
		}
		solutions[sln] = append(solutions[sln], filePath)
	}
	return solutions
}

func (w *Watcher) analyseSolution(sln string) ([]pipeline.AnalyserResult, error) {
	w.solutionLock.Lock()

	if ch, exists := w.solutions[sln]; exists {
		w.solutionLock.Unlock()
		log.Printf("Waiting for another worker to finish analysing %s", sln)
		<-ch
		log.Printf("Another worker finished analysing %s", sln)
		return nil, fmt.Errorf("solution already being analysed")
	}

	w.solutions[sln] = make(chan struct{})
	w.solutionLock.Unlock()

	var results []pipeline.AnalyserResult
	var err error

	defer func() {
		w.solutionLock.Lock()
		close(w.solutions[sln])
		delete(w.solutions, sln)
		w.solutionLock.Unlock()
	}()

	results, err = pipeline.RunCSharpAnalyser(sln)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (w *Watcher) collectSupportedFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(w.config.InputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if router.IsSupported(path) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func findSolution(filePath string) string {
	dir := filepath.Dir(filePath)
	for i := 0; i < 10; i++ {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.sln"))
		if len(matches) > 0 {
			return matches[0]
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func (w *Watcher) gitWatcher() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopEvent:
			return
		case <-ticker.C:
			w.checkGitChanges()
		}
	}
}

func (w *Watcher) checkGitChanges() {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = w.config.InputDir
	output, err := cmd.Output()
	if err != nil {
		return
	}
	changedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, file := range changedFiles {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		fullPath := filepath.Join(w.config.InputDir, file)
		cleanPath := filepath.Clean(fullPath)
		inputDir := filepath.Clean(w.config.InputDir)
		if !strings.HasPrefix(cleanPath, inputDir) {
			continue
		}
		if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
			w.workQueue <- Event{Path: cleanPath, ChangeType: "deleted"}
			continue
		}
		if !router.IsSupported(cleanPath) {
			continue
		}
		w.hashLock.RLock()
		prevHash := w.fileHashes[cleanPath]
		w.hashLock.RUnlock()
		newHash, err := fileHash(cleanPath)
		if err != nil {
			continue
		}
		if prevHash != "" && newHash == prevHash {
			continue
		}
		w.hashLock.Lock()
		w.fileHashes[cleanPath] = newHash
		w.hashLock.Unlock()
		changeType := "modified"
		if prevHash == "" {
			changeType = "created"
		}
		w.workQueue <- Event{Path: cleanPath, ChangeType: changeType}
	}
	cmd = exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = w.config.InputDir
	output, err = cmd.Output()
	if err != nil {
		return
	}
	stagedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, file := range stagedFiles {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		fullPath := filepath.Join(w.config.InputDir, file)
		cleanPath := filepath.Clean(fullPath)
		inputDir := filepath.Clean(w.config.InputDir)
		if !strings.HasPrefix(cleanPath, inputDir) {
			continue
		}
		if !router.IsSupported(cleanPath) {
			continue
		}
		w.hashLock.RLock()
		prevHash := w.fileHashes[cleanPath]
		w.hashLock.RUnlock()
		newHash, err := fileHash(cleanPath)
		if err != nil {
			continue
		}
		if prevHash != "" && newHash == prevHash {
			continue
		}
		w.hashLock.Lock()
		w.fileHashes[cleanPath] = newHash
		w.hashLock.Unlock()
		changeType := "modified"
		if prevHash == "" {
			changeType = "created"
		}
		w.workQueue <- Event{Path: cleanPath, ChangeType: changeType}
	}
}

func (w *Watcher) readEvents() {
	for {
		select {
		case <-w.stopEvent:
			return
		case event := <-w.watcher.Events:
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				w.handleEvent(event)
			}
		case err := <-w.watcher.Errors:
			if err != nil {
				log.Printf("Watcher error: %v", err)
			}
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := filepath.Clean(event.Name)
	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		w.hashLock.Lock()
		delete(w.fileHashes, path)
		w.hashLock.Unlock()
		w.workQueue <- Event{Path: path, ChangeType: "deleted"}
		return
	}
	if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
		w.debouncer.Schedule(path, func() {
			w.hashLock.RLock()
			prevHash := w.fileHashes[path]
			w.hashLock.RUnlock()
			newHash, err := fileHash(path)
			if err != nil {
				return
			}
			if prevHash != "" && newHash == prevHash {
				return
			}
			w.hashLock.Lock()
			w.fileHashes[path] = newHash
			w.hashLock.Unlock()
			changeType := "created"
			if prevHash != "" {
				changeType = "modified"
			}
			w.workQueue <- Event{Path: path, ChangeType: changeType}
		})
	}
}

func (w *Watcher) worker() {
	defer w.wg.Done()
	for {
		select {
		case <-w.stopEvent:
			return
		case ev, ok := <-w.workQueue:
			if !ok {
				return
			}
			w.processEvent(ev)
		}
	}
}

func (w *Watcher) processEvent(ev Event) {
	if ev.ChangeType == "initial" {
		log.Printf("Initial sync processing: %s", ev.Path)
	} else {
		log.Printf("Processing %s: %s", ev.ChangeType, ev.Path)
	}
	if ev.ChangeType == "deleted" {
		if err := w.config.LightRAGConfig.DeleteDocument(ev.Path); err != nil {
			log.Printf("Delete document failed: %v", err)
		}
		if err := w.config.LightRAGConfig.DeleteEntitiesByFilePath(ev.Path); err != nil {
			log.Printf("Delete entities by filepath failed: %v", err)
		}
		return
	}
	if ev.ChangeType == "modified" {
		if err := pipeline.RunEventWithChangeType(ev.Path, w.config.LightRAGConfig, ev.Results, ev.ChangeType); err != nil {
			log.Printf("Pipeline failed for %s: %v", ev.Path, err)
		}
		return
	}
	if err := pipeline.RunEvent(ev.Path, w.config.LightRAGConfig, ev.Results); err != nil {
		log.Printf("Pipeline failed for %s: %v", ev.Path, err)
	}
}

func (w *Watcher) Stop() {
	close(w.stopEvent)
	if w.watcher != nil {
		w.watcher.Close()
	}
	close(w.workQueue)
	w.wg.Wait()
	log.Printf("Watcher: stopped")
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5.Sum(data)), nil
}