# syncr

Lightweight folder sync tool. Uses rclone for sync, Bun for runtime, pm2 for scheduling. Works well with cloud sync tools (OneDrive, Dropbox, Google Drive).

## Use Cases

- **Multi-machine aggregation** - Consolidate files from multiple machines into a single location
- **Documentation site** - Gather `docs` folders from various projects to build a static site (VuePress, Docusaurus, etc.)
- **Build artifact collection** - Track and consolidate build outputs, logs, or reports from different projects
- **Content curation** - Selectively sync specific folders to curate collections of notes, research, or reference materials

## Prerequisites

- **Bun** - [https://bun.sh](https://bun.sh)
- **rclone** - [https://rclone.org/downloads/](https://rclone.org/downloads/)
- **pm2** (optional) - For scheduled sync: `npm install -g pm2`

## Quick Start

```bash
# Install dependencies
bun install

# Check prerequisites
bun run syncr --check-tools

# Initialize configuration
bun run syncr --init

# Edit config/machine-configs/YOUR-MACHINE.json
# Add folder paths, set enabled: true

# Test
bun run syncr --dry-run

# Run sync
bun run syncr
```

## Configuration

Edit `config/machine-configs/MACHINE-NAME.json`:

```json
{
  "machine_name": "MACHINE-NAME",
  "sources": [
    {
      "name": "webapp-docs",
      "path": "C:\\Users\\Peter\\Projects\\webapp\\docs",
      "enabled": true
    },
    {
      "name": "research",
      "path": "D:\\Notes\\research",
      "enabled": true
    }
  ]
}
```

Any folder can be synced - documentation, notes, project files, etc.

## Usage

```bash
# List configured sources
bun run syncr --list-only

# Dry run (preview changes)
bun run syncr --dry-run

# Run sync
bun run syncr

# Verbose output
bun run syncr --verbose

# Check tools
bun run syncr --check-tools

# Show help
bun run syncr --help
```

## How It Works

1. Reads machine-specific configuration from `config/machine-configs/`
2. Syncs each enabled folder to `backups/MACHINE-NAME/` using rclone
3. Each machine has isolated backup folder (no conflicts)

### Backup Location

```
./backups/MACHINE-NAME/source-name/
```

syncr backs up to the `backups/` directory relative to where it runs from.

### Features

- **Cross-platform** - Works on Windows, macOS, Linux
- **Mirror mode** - Destination matches source exactly
- **Checksum verification** - Ensures file integrity
- **Incremental sync** - Only changed files transferred
- **Filter rules** - Excludes temp files, tokens, build artifacts
- **Daily logs** - Track all operations
- **Multi-machine support** - Each machine isolated
- **pm2 integration** - Easy scheduling with cron

## Scheduling with pm2

```bash
# Start hourly sync
pm2 start ecosystem.config.js

# Check status
pm2 status

# View logs
pm2 logs syncr

# Stop
pm2 stop syncr

# Persist across reboots
pm2 startup
pm2 save
```

Edit `ecosystem.config.js` to change the schedule:
```javascript
cron_restart: '0 * * * *'  // Every hour (default)
cron_restart: '*/30 * * * *'  // Every 30 minutes
cron_restart: '0 */2 * * *'  // Every 2 hours
```

## Directory Structure

```
syncr/
+-- src/                     # TypeScript source
|   +-- cli.ts               # CLI entry point
|   +-- backup.ts            # Main sync logic
|   +-- config.ts            # Configuration handling
|   +-- rclone.ts            # rclone integration
|   +-- logger.ts            # Logging
|   +-- types.ts             # TypeScript types
+-- config/
|   +-- filters.txt          # Exclusion rules
|   +-- machine-configs/     # Per-machine configs
+-- logs/                    # Daily logs
+-- backups/                 # Synced data
|   +-- MACHINE-NAME/
+-- package.json
+-- ecosystem.config.js      # pm2 config
```

## Filter Rules

`config/filters.txt` excludes:
- Version control (`.git`, `.svn`)
- Dependencies (`node_modules`, `__pycache__`)
- Build artifacts (`bin/`, `obj/`, `*.exe`)
- Temporary files (`*.tmp`, `*.swp`)
- Token files (`token.txt`, `*.token`)
- Log files (`*.log`)

## Logs

Daily logs in `logs/syncr_YYYYMMDD.log`:
```
[2026-01-07 12:13:25] [INFO] === Starting Sync ===
[2026-01-07 12:13:25] [INFO] Machine: DESKTOP-PC
[2026-01-07 12:13:26] [INFO] Processing: webapp-docs
[2026-01-07 12:13:26] [INFO]   From: C:\Users\Peter\Projects\webapp\docs
[2026-01-07 12:13:26] [INFO]   To:   ./backups/DESKTOP-PC/webapp-docs
[2026-01-07 12:13:27] [SUCCESS]   Status: Success (624ms)
```

## Build Single Binary

```bash
bun build src/cli.ts --compile --outfile dist/syncr
```

Creates a standalone executable that runs without Bun installed.

## Multi-Machine

Each machine:
- Has own config file: `config/machine-configs/HOSTNAME.json`
- Syncs to own subfolder: `backups/HOSTNAME/`
- No conflicts between machines

## Troubleshooting

```bash
# Verify tools installed
bun run syncr --check-tools

# List sources and status
bun run syncr --list-only

# Check pm2 status
pm2 status

# View recent logs
tail -50 logs/syncr_*.log

# Test without changes
bun run syncr --dry-run --verbose
```

### Common Issues

**rclone not found**: Install rclone, ensure it's in PATH

**Source not found**: Verify path exists, check for typos in config

**Config not found**: Run `bun run syncr --init`

**pm2 not found**: Install with `npm install -g pm2`

## Documentation

- [Technical Design](docs/design.md) - Architecture and implementation details
- [AI Agent Guide](AGENTS.md) - Quick reference for contributors
