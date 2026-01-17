# AI Agent Guide

Quick reference for AI agents contributing to this project.

## Project Purpose

Lightweight bidirectional folder sync. Single Go binary with rclone embedded. Works with cloud storage (OneDrive, Dropbox, Google Drive).

## Architecture

```
syncr daemon (continuous loop)
    |
    v
internal/sync/bisync.go --> rclone bisync --> sync_root/{project}/
    |
    +-- reads: syncr.json (config)
    +-- reads/writes: {sync_root}/_syncr/state.json
    +-- logs: {sync_root}/_syncr/logs/syncr_YYYYMMDD.log
```

## Key Files

| File | Purpose |
|------|---------|
| `main.go` | CLI entry point, command dispatch |
| `cmd/init.go` | Initialize project for first sync |
| `cmd/sync.go` | One-shot sync command |
| `cmd/daemon.go` | Continuous sync loop |
| `cmd/status.go` | Show project status, conflicts |
| `cmd/config.go` | Display configuration |
| `internal/config/config.go` | Config loading and validation |
| `internal/state/state.go` | Sync state tracking |
| `internal/sync/bisync.go` | rclone bisync wrapper |
| `internal/sync/conflicts.go` | Conflict file detection |
| `internal/logger/logger.go` | Console and file logging |

## Configuration Format

`syncr.json` in working directory:

```json
{
  "sync_root": "/Users/you/OneDrive/syncr",
  "sync_interval_seconds": 300,
  "projects": [
    {
      "name": "docs",
      "local_path": "/Users/you/Projects/app/docs",
      "sync_path": "docs",
      "enabled": true
    }
  ]
}
```

## Sync Behavior

Uses rclone bisync (bidirectional sync):

```
local_path  <--->  sync_root/sync_path/
```

- **Bidirectional**: Changes sync both ways
- **State tracking**: Remembers last sync, detects conflicts
- **Conflict files**: `*.conflict1` created when same file changes both sides

## Common Tasks

### Add new project

Edit `syncr.json`, add to `projects` array, then:
```bash
syncr init newproject
```

### Test sync without changes

```bash
syncr -dry-run sync
```

### Check status

```bash
syncr status
```

### Modify sync interval

Change `sync_interval_seconds` in `syncr.json` (minimum 60, default 300).

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run integration tests
go test ./tests/...

# Verbose test output
go test -v ./...
```

## Code Conventions

- Go with standard formatting (`go fmt`)
- Internal packages in `internal/`
- Commands in `cmd/`
- Use `logger.Info()`, `logger.Error()`, `logger.Debug()` for logging
- Errors: return error, let caller handle exit
- Context: pass `context.Context` for cancellation

## File Locations

```
syncr/
├── main.go              # Entry point
├── cmd/                 # CLI commands
├── internal/
│   ├── config/          # Configuration
│   ├── state/           # State tracking
│   ├── sync/            # Bisync wrapper
│   └── logger/          # Logging
├── tests/               # Integration tests
├── _docs/               # Design docs, plans
└── syncr.json           # User config (not in repo)
```

Cloud storage structure:
```
{sync_root}/
├── _syncr/
│   ├── state.json       # Sync state
│   ├── logs/            # Log files
│   └── bisync/          # rclone working data
├── {project1}/          # Synced files
└── {project2}/          # Synced files
```

## Dependencies

- **Go 1.21+**: Build and development
- **rclone** (embedded): No external install needed

## Building

```bash
# Development build
go build -o syncr .

# Run directly
go run . status

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o syncr-linux-amd64 .

# With version info
go build -ldflags "-X main.version=v1.0.0 -X main.commit=$(git rev-parse --short HEAD)" -o syncr .
```

## Quick Validation

Run `./scripts/pre-push.sh` before committing - it checks formatting, vet, build, and tests.

```bash
# Build and check help
go build -o syncr . && ./syncr help

# Run tests
go test ./...

# Check a specific package
go test -v ./internal/config/
```

NOTE: The user may accidentally say "rsync" instead of "syncr" - clarify if needed.
