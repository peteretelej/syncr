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
    +-- reads/writes: ~/.config/syncr/state.json
    +-- logs: ~/.config/syncr/logs/syncr_YYYYMMDD.log
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
| `internal/discover/` | Folder discovery scan, planning, and state |
| `internal/config/config.go` | Config loading and validation |
| `internal/state/state.go` | Sync state tracking |
| `internal/sync/bisync.go` | rclone bisync wrapper |
| `internal/sync/conflicts.go` | Conflict file detection |
| `internal/sync/trash.go` | Trash cleanup and stats |
| `internal/logger/logger.go` | Console and file logging |

## Configuration Format

`syncr.json` in working directory:

```json
{
  "sync_root": "/Users/you/OneDrive/syncr",
  "sync_interval_minutes": 5,
  "backup_retention_days": 30,
  "conflict_resolve": "newer",
  "conflict_suffix": "{DateOnly}",
  "discover": {
    "scan_roots": ["/Users/you/Projects"],
    "folder_names": ["_docs", "_scratch", "_planning", "prds"],
    "exclude_globs": [".worktrees", "node_modules"],
    "scan_interval_hours": 24
  },
  "projects": [
    {
      "name": "docs",
      "local_path": "/Users/you/Projects/app/docs",
      "sync_path": "docs",
      "enabled": true,
      "backup_dir": true,
      "conflict_resolve": "path1",
      "exclude": ["*.db", ".cache/"],
      "hooks": {
        "post_sync": "./rebuild.sh",
        "on_conflict": "echo 'conflict detected'"
      },
      "hook_timeout_seconds": 60,
      "derived": {
        "*.db": "Search index, rebuilt by post_sync hook"
      }
    }
  ]
}
```

### Conflict Resolution Fields

| Field | Level | Description |
|-------|-------|-------------|
| `conflict_resolve` | global + project | Resolution strategy: `none`, `newer`, `older`, `larger`, `smaller`, `path1`, `path2`. Project overrides global. |
| `conflict_suffix` | global | Custom suffix for conflict files. Supports rclone time globs: `{DateOnly}`, `{TimeOnly}`, `{DateTimeISO}`. Default: `conflict`. |

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

### Preview folder discovery

```bash
syncr -dry-run discover
```

### Check status

```bash
syncr status
```

### Modify sync interval

Change `sync_interval_minutes` in `syncr.json` (minimum 1, default 5).

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
└── syncr.json           # User config (not in repo)
```

Sync folder structure:
```
{sync_root}/
├── {project1}/          # Synced files
├── {project2}/          # Synced files
└── _syncr/trash/        # Backup copies (when backup_dir enabled)
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
