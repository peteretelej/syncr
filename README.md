# syncr

Lightweight bidirectional folder sync.

Keeps local folders in sync with cloud storage (OneDrive, Dropbox, Google Drive). Single binary, zero dependencies.

## What It Does

- **Bidirectional sync** - changes flow both ways between local and cloud
- **State tracking** - remembers sync history, detects conflicts
- **Daemon mode** - continuous background sync
- **Multi-project** - sync multiple folders with one config
- **Cross-platform** - macOS, Linux, Windows

## Install

Download the binary for your platform from [Releases](https://github.com/peteretelej/syncr/releases), or build from source:

```bash
go install github.com/peteretelej/syncr@latest
```

## Quick Start

```bash
# Create config file
cat > syncr.json << 'EOF'
{
  "cloud_root": "/path/to/OneDrive/syncr",
  "sync_interval_seconds": 300,
  "projects": [
    {
      "name": "docs",
      "local_path": "/path/to/local/docs",
      "cloud_subpath": "docs",
      "enabled": true
    }
  ]
}
EOF

# Initialize project (required before first sync)
syncr init docs

# Run sync
syncr sync

# Or run continuous sync
syncr daemon
```

## Configuration

Create `syncr.json` in your working directory:

```json
{
  "cloud_root": "/Users/you/OneDrive/syncr",
  "sync_interval_seconds": 300,
  "projects": [
    {
      "name": "project-docs",
      "local_path": "/Users/you/Projects/myapp/docs",
      "cloud_subpath": "project-docs",
      "enabled": true
    },
    {
      "name": "notes",
      "local_path": "/Users/you/Notes",
      "cloud_subpath": "notes",
      "enabled": true
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `cloud_root` | Base path in your cloud storage folder |
| `sync_interval_seconds` | How often daemon syncs (default: 300) |
| `projects[].name` | Project identifier |
| `projects[].local_path` | Absolute path to local folder |
| `projects[].cloud_subpath` | Subfolder name in cloud_root |
| `projects[].enabled` | Set false to skip this project |

## Commands

```bash
syncr init <project>     # Initialize project for first sync
syncr sync               # Sync all enabled projects
syncr sync <project>     # Sync specific project
syncr daemon             # Run continuous sync
syncr status             # Show project status and conflicts
syncr config             # Show current configuration
syncr version            # Show version
```

### Options

```
-config string    Path to config file (default: ./syncr.json)
-verbose          Enable verbose output
-dry-run          Show what would sync without making changes
```

## How It Works

syncr uses rclone's bisync under the hood. Files sync bidirectionally:

```
Local Folder  <--->  Cloud Storage
~/docs/       <--->  OneDrive/syncr/docs/
```

State and logs are stored in `{cloud_root}/_syncr/`:
- `state.json` - sync history, initialization status
- `logs/` - daily log files
- `bisync/` - rclone bisync working data

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
│   ├── config/          # Configuration loading
│   ├── state/           # Sync state tracking
│   ├── sync/            # rclone bisync wrapper
│   └── logger/          # Logging
├── syncr.json           # Your config (create this)
└── tests/               # Integration tests
```

## License

MIT
