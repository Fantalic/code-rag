# Phase 36 — Deploy-Ready CLI

## Current Task: Phase 36.1 — CLI Polish & UX

Make `coderag` feel like a polished production CLI. Fix signal handling so Ctrl+C works, add `--version`, add `--quiet`/`--verbose` flags, add spinners for waiting operations, progress bars for scans, and clean up `coderag status` output.

## Issues Found

| # | Issue | Where | Impact |
|---|-------|-------|--------|
| 1 | `signal.Notify` **never called** — `<-sigCh` blocks forever, Ctrl+C does nothing | `cmd/coderag/main.go:299,368` | User cannot stop `coderag start/watch` gracefully |
| 2 | No `--version` flag | `cli.App{}` | No way to check installed version |
| 3 | No `--quiet` / `--verbose` | All commands | All output uses `log.Printf` — raw timestamps, no levels |
| 4 | No progress indicators | `ensureLightRAG`, `ensureMCP` | "Waiting for LightRAG to become healthy..." with no spinner |
| 5 | Scan has no progress bar | `scanCmd` | "Analyser produced 348 output files" + raw logs, no % progress |
| 6 | `coderag status` uses `log.Printf` | `statusCmd` | Shows as timestamped log lines instead of a clean table |
| 7 | Docker compose output spills to stdout | `startCmd`, `scanCmd` | `docker compose up -d` output mixed into CLI output |
| 8 | `openBrowser()` missing Linux/macOS | `cmd/coderag/main.go:320` | Only `rundll32` (Windows) and `wslview` (WSL) — no `xdg-open`/`open` |
| 9 | Duplicate signal/startup logic | `startCmd` & `watchCmd` | Both have identical Docker+watcher setup duplicated |

## Sub-tasks

| # | Step | What changes | Files |
|---|------|-------------|-------|
| 1 | Fix Ctrl+C — add `signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)` | `startCmd` and `watchCmd` stop on Ctrl+C | `cmd/coderag/main.go` |
| 2 | Add `--version` — inject via `ldflags`, enable `cli.VersionFlag` | `coderag --version` prints version | `cmd/coderag/main.go`, `Makefile`/`scripts` |
| 3 | Add `--quiet` / `--verbose` global flags | Filter log output by level, suppress info in quiet mode | `cmd/coderag/main.go` |
| 4 | Add spinner for Docker health polling | Replace "Waiting for LightRAG to become healthy..." with animated spinner | `cmd/coderag/main.go` |
| 5 | Add progress bar for scan | Show entity/relation extraction + import progress | `cmd/coderag/main.go`, `pkg/watchdog/pipeline/run.go` |
| 6 | Clean up `coderag status` output | Table format: services status, workspace, entity count, uptime | `cmd/coderag/main.go` |
| 7 | Suppress Docker compose stdout | Redirect to stderr or log file, not mixed with CLI output | `cmd/coderag/main.go` |
| 8 | Fix `openBrowser` for Linux/macOS | `xdg-open` (Linux), `open` (macOS), `cmd /c start` (Windows) | `cmd/coderag/main.go` |
| 9 | Extract shared Docker+watcher startup | DRY up `startCmd` and `watchCmd` duplicated setup | `cmd/coderag/main.go` |

## Dependencies to add

```
go get github.com/briandowns/spinner   # spinners for waits
go get github.com/schollz/progressbar/v3  # progress bars for scans
```

## Phase 36.2 — Installation & Distribution (NEXT)

| # | Step | What changes | Files |
|---|------|-------------|-------|
| 1 | `coderag init <dir>` | Scaffold `.coderag/`, `.env`, detect project type | `cmd/coderag/main.go` |
| 2 | Embedded `csharp_analyser` | Use `//go:embed` to bundle analyser binary into `coderag` binary | `cmd/coderag/main.go`, build scripts |
| 3 | `scripts/install.sh` | One-command: detect platform, download pre-built binary from GitHub | `scripts/install.sh` |
| 4 | `scripts/install.ps1` | Same for Windows | `scripts/install.ps1` |
| 5 | `.github/workflows/release.yml` | Build for linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64 on tag | `.github/workflows/release.yml` |
| 6 | `coderag upgrade` | Self-update: check latest release, download + replace binary | `cmd/coderag/main.go` |

## Phase 36.3 — Logging Overhaul (NEXT)

| # | Step | What changes |
|---|------|-------------|
| 1 | Replace `log.Printf` with `slog` (Go 1.21+ stdlib) | Structured logging with levels (DEBUG, INFO, WARN, ERROR) |
| 2 | Normal mode: show only summary + warnings | Suppress `"Analyser produced 348 output files"`-style noise |
| 3 | Verbose mode: show debug details + HTTP tracing | All current `log.Printf` output |
| 4 | Consistent error format | `ERROR [component] message` or `WARN [component] message` |

## Phase 36.4 — Cross-Platform Readiness (NEXT)

| # | Step | What changes |
|---|------|-------------|
| 1 | `openBrowser` for Linux/macOS | `xdg-open` / `open` |
| 2 | Shell completion | `coderag completion bash` / `zsh` / `fish` / `powershell` |

## Phase 36.5 — Configuration (NEXT)

| # | Step | What changes |
|---|------|-------------|
| 1 | `coderag config show` | Print current config (env vars + defaults) |
| 2 | `coderag config set KEY=VALUE` | Update `.coderag/.env` |
| 3 | `coderag config init` | Generate `.env.example` |
| 4 | XDG config on Linux | `~/.config/coderag/config.env` for global defaults |

## Phase 36.6 — General Polish (NEXT)

| # | Step | What changes |
|---|------|-------------|
| 1 | Colorized output | Green ✓ / Yellow ⚠ / Red ✗ for command results |
| 2 | Summary at end of long ops | "✔ Scan complete: 1784 entities, 1853 relations in 59.25s" |
| 3 | `coderag upgrade` | Self-update mechanism |
