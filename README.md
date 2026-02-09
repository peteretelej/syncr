# syncr

Lightweight bidirectional folder sync.

Keeps local folders in sync with cloud storage (OneDrive, Dropbox, Google Drive). Single binary, zero dependencies.

## What It Does

- **Bidirectional sync** - changes flow both ways between local and cloud
- **State tracking** - remembers sync history, detects conflicts
- **Daemon mode** - continuous background sync with config hot-reload
- **Multi-project** - sync multiple folders with one config
- **Cross-platform** - macOS, Linux, Windows

## Install

Download the binary for your platform from [Releases](https://github.com/peteretelej/syncr/releases), or build from source:

```bash
go install github.com/peteretelej/syncr@latest
```

## Quick Start

```bash
# Generate a starter config file
syncr init

# Edit syncr.json: set sync_root to your cloud storage path,
# then update the sample project or add your own

# Add a project
syncr add docs ~/Projects/myapp/docs

# Run sync
syncr sync

# Or run continuous sync
syncr daemon
```

You can also create `syncr.json` manually:

```json
{
  "sync_root": "/Users/You/OneDrive/syncr",
  "sync_interval_seconds": 300,
  "projects": [
    {
      "name": "docs",
      "local_path": "/Users/You/Projects/myapp/docs",
      "sync_path": "docs",
      "enabled": true
    }
  ]
}
```

Then initialize and sync:

```bash
syncr init docs
syncr sync
```

## Configuration

```json
{
  "sync_root": "/Users/You/OneDrive/syncr",
  "sync_interval_seconds": 300,
  "projects": [
    {
      "name": "project-docs",
      "local_path": "/Users/You/Projects/myapp/docs",
      "sync_path": "project-docs",
      "enabled": true
    },
    {
      "name": "notes",
      "local_path": "/Users/You/Notes",
      "sync_path": "notes",
      "enabled": true
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `sync_root` | Base path in your cloud storage folder |
| `sync_interval_seconds` | How often daemon syncs (minimum 60, default 300) |
| `projects[].name` | Project identifier |
| `projects[].local_path` | Absolute path to local folder |
| `projects[].sync_path` | Subfolder name in sync_root |
| `projects[].enabled` | Set false to skip this project |

## Commands

```bash
syncr init [project]       # Initialize project(s) for first sync
syncr init                 # Initialize all uninitialized enabled projects
syncr add <name> [path]    # Add a new project and initialize it
syncr sync [project]       # Sync all enabled projects (or a specific one)
syncr daemon               # Run continuous sync
syncr status               # Show project status and conflicts
syncr config               # Show current configuration and validation
syncr logs                 # Show today's log
syncr logs -f              # Follow log output in real time
syncr enable <project>     # Enable a project for syncing
syncr disable <project>    # Disable a project from syncing
syncr version              # Show version
```

### Options

```
-config, -c string    Path to config file (default: ./syncr.json)
-verbose              Enable verbose output
-dry-run              Show what would sync without making changes
```

## How It Works

syncr uses rclone's bisync under the hood. Files sync bidirectionally:

```
Local Folder                      Cloud Storage
~/Projects/myapp/docs  <------>  OneDrive/syncr/docs/
```

Sync metadata is stored in `{sync_root}/_syncr/`:
- `logs/` - daily log files (auto-rotated, 7-day retention)
- `bisync/` - rclone bisync working data

Sync state (`state.json`) is stored locally per machine in your OS config directory (e.g. `~/.config/syncr/` on Linux/macOS), so each machine tracks its own sync history independently.

## Initialization

Before first sync, each project must be initialized:

```bash
syncr init myproject
```

This handles the initial sync based on what exists:
- **Local empty, cloud has files**: pulls from cloud
- **Cloud empty, local has files**: pushes to cloud
- **Both have files**: merges (keeps superset)
- **Both empty**: marks initialized, nothing to sync

To initialize all uninitialized enabled projects at once:

```bash
syncr init
```

## Conflicts

When the same file changes in both locations, rclone creates conflict files (e.g., `file.txt.conflict1`). Check for conflicts:

```bash
syncr status
```

Resolve conflicts manually by keeping the version you want.

## Build from Source

```bash
git clone https://github.com/peteretelej/syncr
cd syncr
go build -o syncr .
```

Cross-compile:

```bash
GOOS=darwin GOARCH=arm64 go build -o syncr-darwin-arm64 .
GOOS=linux GOARCH=amd64 go build -o syncr-linux-amd64 .
GOOS=windows GOARCH=amd64 go build -o syncr-windows-amd64.exe .
```

## Project Structure

```
syncr/
├── main.go              # CLI entry point
├── cmd/                 # Command implementations
├── internal/
│   ├── config/          # Configuration loading and validation
│   ├── state/           # Sync state tracking
│   ├── sync/            # rclone bisync wrapper
│   ├── progress/        # Sync progress output
│   └── logger/          # Logging with daily rotation
├── syncr.example.json   # Example config
├── syncr.json           # Your config (create this, gitignored)
└── tests/               # Integration tests
```

## License

MIT
