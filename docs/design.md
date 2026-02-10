# syncr - Technical Design

## Overview

Lightweight bidirectional folder sync. Single Go binary with rclone embedded. Works with cloud storage (OneDrive, Dropbox, Google Drive).

## Architecture

```
+---------------------+
|  syncr daemon       |
|  (continuous loop)  |
+----------+----------+
           |
           v
+---------------------+
|  internal/sync      |
|  bisync.go          |
+----------+----------+
           |
           +---> Config (syncr.json)
           |
           +---> State (~/.config/syncr/state.json)
           |
           +---> rclone bisync (embedded library)
           |
           +---> {sync_root}/{project}/
```

## Data Flow

```
Local Folders                Sync Folder
    |                            |
    +-> ~/Projects/app/docs      |
    +-> ~/Notes/research    <--> +-> OneDrive/syncr/docs/
                                 +-> OneDrive/syncr/research/
         |
         +---> [rclone bisync - bidirectional]
         |
         +---> State tracked in ~/.config/syncr/state.json
```

## Directory Structure

### Source Code

```
syncr/
├── main.go                  # CLI entry point, command dispatch
├── cmd/
│   ├── init.go              # Initialize project
│   ├── sync.go              # One-shot sync
│   ├── daemon.go            # Continuous sync loop
│   ├── status.go            # Show status and conflicts
│   ├── config.go            # Display configuration
│   └── version.go           # Version info
├── internal/
│   ├── config/
│   │   ├── config.go        # Config loading and validation
│   │   └── config_test.go
│   ├── state/
│   │   ├── state.go         # Sync state management
│   │   └── state_test.go
│   ├── sync/
│   │   ├── bisync.go        # rclone bisync wrapper
│   │   ├── conflicts.go     # Conflict detection
│   │   └── *_test.go
│   └── logger/
│       ├── logger.go        # Console and file logging
│       └── logger_test.go
├── tests/                   # Integration tests
├── go.mod
└── go.sum
```

### Runtime Locations

User config (not in repo):
```
./syncr.json                 # Working directory
```

Sync folder:
```
{sync_root}/
├── {project1}/              # Synced files
└── {project2}/              # Synced files
```

## Package Design

### main.go

Entry point with flag parsing and command dispatch.

```go
// Global flags
-config string    // Path to config file
-verbose          // Enable verbose output
-dry-run          // Preview without changes

// Commands
init <project>    // Initialize for first sync
sync [project]    // One-shot sync
daemon            // Continuous sync
status            // Show status
config            // Show configuration
version           // Show version
```

### internal/config

Configuration loading and validation.

```go
type Config struct {
    SyncRoot            string    `json:"sync_root"`
    SyncIntervalMinutes int       `json:"sync_interval_minutes"`
    Projects            []Project `json:"projects"`
}

type Project struct {
    Name         string `json:"name"`
    LocalPath    string `json:"local_path"`
    SyncPath string `json:"sync_path"`
    Enabled      bool   `json:"enabled"`
}
```

Responsibilities:
- Load from `syncr.json` or `-config` path
- Validate paths exist and are absolute
- Apply defaults (sync interval = 5 minutes)
- Compute `SyncrDataDir()` for local state storage

### internal/state

Sync state tracking. State is stored locally per-machine in `~/.config/syncr/`.

```go
type State struct {
    Version   int                     `json:"version"`
    MachineID string                  `json:"machine_id"`
    Projects  map[string]ProjectState `json:"projects"`
}

type ProjectState struct {
    Initialized    bool      `json:"initialized"`
    InitializedAt  time.Time `json:"initialized_at,omitempty"`
    LastSync       time.Time `json:"last_sync,omitempty"`
    LastSyncStatus string    `json:"last_sync_status,omitempty"`
    SyncCount      int       `json:"sync_count"`
    ErrorCount     int       `json:"error_count"`
    LastError      string    `json:"last_error,omitempty"`
}
```

Responsibilities:
- Load/save state with atomic writes
- Track initialization status per project
- Record sync success/failure
- Thread-safe via mutex

### internal/sync

rclone bisync wrapper.

```go
type BisyncOptions struct {
    Resync       bool
    ResyncMode   string // "path1", "path2", or ""
    DryRun       bool
    Verbose      bool
    SyncrDataDir string
}

type BisyncResult struct {
    Transferred int
    Deleted     int
    Errors      int
    Duration    time.Duration
}

func RunBisync(ctx context.Context, localPath, cloudPath string, opts BisyncOptions) (*BisyncResult, error)
```

Bisync options applied:
- `--compare size,modtime`
- `--max-delete 50%`
- `--resilient`
- `--recover`
- `--workdir {syncrDataDir}/bisync`

Conflict detection:
```go
func CountConflicts(path string) (int, error)
func ListConflicts(path string) ([]string, error)
```

### internal/logger

Logging with console and file output.

```go
type Logger struct {
    out     io.Writer
    file    *os.File
    verbose bool
}

func (l *Logger) Info(format string, args ...interface{})
func (l *Logger) Warn(format string, args ...interface{})
func (l *Logger) Error(format string, args ...interface{})
func (l *Logger) Debug(format string, args ...interface{})  // verbose only
```

Log file: `{syncrDataDir}/logs/syncr_YYYYMMDD.log`

Format:
```
[2026-01-17 10:30:00] [INFO] Message here
```

## Configuration Schema

```json
{
  "sync_root": "/Users/you/OneDrive/syncr",
  "sync_interval_minutes": 5,
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

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `sync_root` | string | required | Base path for sync folder |
| `sync_interval_minutes` | int | 5 | Daemon sync interval in minutes (min 1) |
| `projects` | array | required | List of projects to sync |
| `projects[].name` | string | required | Project identifier |
| `projects[].local_path` | string | required | Absolute local path |
| `projects[].sync_path` | string | required | Subfolder in sync_root |
| `projects[].enabled` | bool | true | Include in sync |

## Initialization

Before first sync, projects must be initialized. The init command handles different starting states:

| Local | Sync Folder | Action |
|-------|-------------|--------|
| Empty | Has files | Resync with `--resync-mode path2` (sync folder wins) |
| Has files | Empty | Resync with `--resync-mode path1` (local wins) |
| Empty | Empty | Mark initialized, nothing to sync |
| Has files | Has files | Resync (files from both sides will be kept) |

## Daemon Mode

Continuous sync loop:
1. Initial sync on startup
2. Wait for `sync_interval_minutes`
3. Sync all enabled, initialized projects
4. Handle SIGINT/SIGTERM for graceful shutdown
5. Write PID file to `~/.config/syncr/syncr.pid`

Error handling:
- Skip uninitialized projects
- Skip projects that hit max consecutive errors (5)
- Suggest `syncr init --force` after error threshold

## Conflict Handling

rclone bisync creates conflict files when the same file changes in both locations:
```
file.txt.conflict1
file.txt.conflict2
```

The `status` command counts and lists conflicts. Resolution is manual - user keeps the version they want and deletes conflict files.

## Error Handling

| Error | Behavior |
|-------|----------|
| Config not found | Exit with error message |
| Invalid config | Exit with validation errors |
| Missing local path | Skip project, log warning |
| Missing sync folder path | Create directory |
| Bisync failure | Log error, continue to next project |
| Max errors hit | Skip project, suggest re-init |

## Building

### Development

```bash
go build -o syncr .
go test ./...
```

### Release

```bash
VERSION=$(git describe --tags --always)
COMMIT=$(git rev-parse --short HEAD)
LDFLAGS="-X main.version=$VERSION -X main.commit=$COMMIT"

go build -ldflags "$LDFLAGS" -o syncr .
```

### Cross-Compile

```bash
GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/syncr-darwin-amd64 .
GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/syncr-darwin-arm64 .
GOOS=linux GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/syncr-linux-amd64 .
GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/syncr-windows-amd64.exe .
```

## Dependencies

rclone is embedded as a Go library - no external installation needed.

```
github.com/rclone/rclone  # Sync engine (bisync)
```

## Limitations

- Conflicts require manual resolution
- No file versioning (use cloud provider's versioning)
- No encryption (relies on cloud provider)
- Requires cloud sync to complete before bisync sees changes
