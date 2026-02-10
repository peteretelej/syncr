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

## Usage

```
syncr - Lightweight bidirectional folder sync

Usage:
  syncr <command> [options]

Commands:
  init [project]      Initialize project(s) (all uninitialized if no name given)
  add <name> [path]   Add a new project interactively
  sync [project]      Run sync once (all projects if no name given)
  daemon              Run continuous sync daemon
  status              Show status of all projects
  config              Show current configuration
  logs                Show today's log (use -f to follow)
  enable <project>    Enable a project for syncing
  disable <project>   Disable a project from syncing
  version             Show version information
  help                Show this help message

Options:
  -config, -c string  Path to config file (default: $SYNCR_CONFIG or ./syncr.json)
  -verbose          Enable verbose output
  -dry-run          Show what would be synced without making changes

Environment Variables:
  SYNCR_CONFIG    Path to config file (overridden by -config flag)

Examples:
  syncr init                   Initialize all uninitialized enabled projects
  syncr init MyProject        Initialize a specific project
  syncr add docs ~/Projects/docs   Add and initialize a project
  syncr sync                  Sync all enabled projects
  syncr sync MyProject        Sync specific project
  syncr daemon                Run continuous sync every 5 minutes
  syncr status                Show project status and conflicts
  syncr enable MyProject      Enable a project for syncing
  syncr disable MyProject     Disable a project from syncing
```

## Configuration

syncr looks for its config file in this order:

1. `-config` / `-c` flag (explicit path)
2. `SYNCR_CONFIG` environment variable
3. `./syncr.json` (current working directory)

Set `SYNCR_CONFIG` to use a shared config from a fixed location:

```bash
export SYNCR_CONFIG=~/syncr.json
```

### Config format

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

| Field | Description |
|-------|-------------|
| `sync_root` | Base path in your cloud storage folder |
| `sync_interval_seconds` | How often daemon syncs (minimum 60, default 300) |
| `projects[].name` | Project identifier |
| `projects[].local_path` | Absolute path to local folder |
| `projects[].sync_path` | Subfolder name under sync_root |
| `projects[].enabled` | Set false to skip this project |

## How It Works

syncr uses rclone's bisync under the hood. Files sync bidirectionally:

```
Local Folder                      Cloud Storage
~/Projects/myapp/docs  <------>  OneDrive/syncr/docs/
```

**Local storage.** Logs, bisync working data, PID file, and sync state are all stored locally on your machine in your OS config directory, not in the cloud sync_root. This means each machine tracks its own sync history independently.

| OS | Local data path |
|----|----------------|
| macOS | `~/Library/Application Support/syncr/` |
| Linux | `~/.config/syncr/` |
| Windows | `%AppData%\syncr\` |

Within that directory:

- `state.json` - per-machine sync state (initialization status, last sync time, error counts)
- `logs/` - daily log files (auto-rotated, 7-day retention)
- `bisync/` - rclone bisync working data

**Safety limits.** Sync aborts if more than 50% of files would be deleted, protecting against accidental bulk deletion.

**Cloud storage** only contains the synced project files:

```
{sync_root}/
├── docs/       # synced project files
└── notes/      # another project
```

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

## License

MIT
