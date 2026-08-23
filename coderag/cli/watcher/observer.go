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
	"github.com/coderag/crud"
	"github.com/coderag/pipeline"
	"github.com/coderag/router"
	"github.com/coderag/status"
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

	// Build hash map for all files (needed for future change detection)
	for _, filePath := range files {
		hash, err := fileHash(filePath)
		if err != nil {
			continue
		}
		w.hashLock.Lock()
		w.fileHashes[filePath] = hash
		w.hashLock.Unlock()
	}

	changedFiles, err := w.getGitChangedFiles(files)
	if err != nil {
		log.Printf("Initial sync: git diff unavailable, processing all files: %v", err)
		changedFiles = files
	}

	deletedFiles := w.getGitDeletedFiles()

	// Handle deleted files: clean up + cascade to affected files
	if len(deletedFiles) > 0 {
		log.Printf("Initial sync: %d file(s) deleted since last commit", len(deletedFiles))
		for _, delPath := range deletedFiles {
			w.queueDeletionWithCascade(delPath)
		}
	}

	if len(changedFiles) == 0 {
		log.Printf("Initial sync: no changed files detected, skipping processing")
		return nil
	}

	log.Printf("Initial sync: %d file(s) changed since last commit", len(changedFiles))
	status.Default.Stage("watch", "initial sync: %d changed file(s)", len(changedFiles))

	solutions := w.groupFilesBySolution(changedFiles)
	log.Printf("Initial sync: found %d solution(s) with changes", len(solutions))

	for sln, slnFiles := range solutions {
		results, err := w.analyseSolution(sln)
		if err != nil {
			log.Printf("Initial sync: analyser failed for %s: %v", sln, err)
			continue
		}

		for _, filePath := range slnFiles {
			w.workQueue <- Event{
				Path:       filePath,
				ChangeType: "modified",
				Results:    results,
			}
		}
	}

	log.Printf("Initial sync: enqueued %d changed file(s) for processing", len(changedFiles))
	return nil
}

func (w *Watcher) getGitChangedFiles(files []string) ([]string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = w.config.InputDir
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("no git history")
	}

	cmd = exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = w.config.InputDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	diffSet := make(map[string]bool)
	rawLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fullPath := filepath.Join(w.config.InputDir, line)
		diffSet[filepath.Clean(fullPath)] = true
	}

	// Also check staged changes
	cmd = exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = w.config.InputDir
	output, err = cmd.Output()
	if err == nil {
		stagedLines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range stagedLines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fullPath := filepath.Join(w.config.InputDir, line)
			diffSet[filepath.Clean(fullPath)] = true
		}
	}

	var result []string
	for _, filePath := range files {
		cleanPath := filepath.Clean(filePath)
		if diffSet[cleanPath] {
			result = append(result, cleanPath)
		}
	}
	return result, nil
}

func (w *Watcher) getGitDeletedFiles() []string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = w.config.InputDir
	if err := cmd.Run(); err != nil {
		return nil
	}

	cmd = exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = w.config.InputDir
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var deleted []string
	rawLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fullPath := filepath.Join(w.config.InputDir, line)
		cleanPath := filepath.Clean(fullPath)
		if _, statErr := os.Stat(cleanPath); os.IsNotExist(statErr) {
			if router.IsSupported(cleanPath) {
				deleted = append(deleted, cleanPath)
			}
		}
	}
	return deleted
}

func (w *Watcher) queueDeletionWithCascade(delPath string) {
	w.hashLock.Lock()
	delete(w.fileHashes, delPath)
	w.hashLock.Unlock()

	// Find entities that belonged to the deleted file
	names, err := w.config.LightRAGConfig.GetEntitiesByFilePath(delPath)
	if err != nil {
		log.Printf("Cascade: failed to get entities for %s: %v", delPath, err)
	} else if len(names) > 0 {
		log.Printf("Cascade: %s defined %d entities, checking for affected files", delPath, len(names))
		affected, affErr := w.findAffectedFiles(names, delPath)
		if affErr == nil && len(affected) > 0 {
			log.Printf("Cascade: %d file(s) reference deleted entities, queuing for re-scan", len(affected))
			for _, affPath := range affected {
				// Schedule re-scan of affected file — will trigger analyser for its solution
				sln := findSolution(affPath)
				if sln != "" {
					results, resErr := w.analyseSolution(sln)
					if resErr != nil {
						log.Printf("Cascade: analyser failed for %s: %v", affPath, resErr)
						continue
					}
					w.workQueue <- Event{
						Path:       affPath,
						ChangeType: "modified",
						Results:    results,
					}
				}
			}
		}
	}

	// Queue the deletion itself
	w.workQueue <- Event{Path: delPath, ChangeType: "deleted"}
}

func (w *Watcher) findAffectedFiles(entityNames []string, excludePath string) ([]string, error) {
	relations, err := w.config.LightRAGConfig.GetAllRelations()
	if err != nil {
		return nil, fmt.Errorf("get all relations: %w", err)
	}

	// Build entity_name -> file_path map from all entities (fallback when relation's
	// source_id/file_path is wrong or empty — e.g. relations stored before the parser fix)
	entityMap := make(map[string]string)
	if nodes, err := w.config.LightRAGConfig.GetAllEntities(); err == nil {
		for _, node := range nodes {
			if props, ok := node["properties"].(map[string]any); ok {
				name, _ := props["entity_name"].(string)
				fp, _ := props["file_path"].(string)
				if name != "" && fp != "" {
					entityMap[name] = fp
				}
			}
		}
	}

	return findAffectedFilesFromData(relations, entityMap, entityNames, excludePath), nil
}

// findAffectedFilesFromData is the pure-logic core of cascade detection.
// It processes relation edges and entity map to find files (other than excludePath)
// that reference deleted entity names.
func findAffectedFilesFromData(relations []crud.RelationEdge, entityMap map[string]string, entityNames []string, excludePath string) []string {
	entitySet := make(map[string]bool, len(entityNames))
	for _, name := range entityNames {
		entitySet[name] = true
	}

	affectedSet := make(map[string]bool)
	for _, rel := range relations {
		if !entitySet[rel.Source] && !entitySet[rel.Target] {
			continue
		}

		// Try relation's own source_id/file_path first (correct since parser fix)
		if rel.SourceID != "" && rel.SourceID != excludePath {
			affectedSet[rel.SourceID] = true
		}
		if rel.FilePath != "" && rel.FilePath != excludePath {
			affectedSet[rel.FilePath] = true
		}

		// Entity-based fallback: find the surviving entity's file_path
		survivor := ""
		if entitySet[rel.Source] {
			survivor = rel.Target
		} else {
			survivor = rel.Source
		}
		if fp, ok := entityMap[survivor]; ok && fp != excludePath {
			affectedSet[fp] = true
		}
	}

	var result []string
	for fp := range affectedSet {
		result = append(result, fp)
	}
	return result
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

	results, err = pipeline.RunCSharpAnalyser(sln, "")
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
			w.queueDeletionWithCascade(cleanPath)
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
		w.queueDeletionWithCascade(path)
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
	status.Default.Detail("debounce", ev.Path, "processing %s change", ev.ChangeType)
	if ev.ChangeType == "deleted" {
		status.Default.Stage("crud", "removing entities of deleted file")
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
			status.Default.Error("scan", "pipeline failed for %s: %v", ev.Path, err)
		}
		return
	}
	if err := pipeline.RunEvent(ev.Path, w.config.LightRAGConfig, ev.Results); err != nil {
		log.Printf("Pipeline failed for %s: %v", ev.Path, err)
		status.Default.Error("scan", "pipeline failed for %s: %v", ev.Path, err)
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