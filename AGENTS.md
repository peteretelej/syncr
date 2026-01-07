# AI Agent Guide

Quick reference for AI agents contributing to this project.

## Project Purpose

Lightweight folder sync tool. Uses rclone for sync, Bun for runtime, pm2 for scheduling. Works well with cloud sync tools (OneDrive, Dropbox, Google Drive).

## Architecture

```
pm2 (hourly cron)
    |
    v
src/cli.ts --> rclone sync --> ./backups/HOSTNAME/
    |
    +-- reads: config/machine-configs/{HOSTNAME}.json
    +-- writes: backups/{HOSTNAME}/{source-name}/
    +-- logs: logs/syncr_YYYYMMDD.log
```

## Key Files

| File | Purpose |
|------|---------|
| `src/cli.ts` | CLI entry point (Commander.js) |
| `src/backup.ts` | Main sync orchestration |
| `src/config.ts` | Config loading and path resolution |
| `src/rclone.ts` | rclone command builder and executor |
| `src/logger.ts` | Console and file logging |
| `src/types.ts` | TypeScript interfaces |
| `config/machine-configs/*.json` | Per-machine source configurations |
| `config/filters.txt` | rclone filter rules (exclusions) |
| `ecosystem.config.js` | pm2 scheduling configuration |
| `docs/design.md` | Technical architecture details |

## Configuration Format

```json
{
  "machine_name": "HOSTNAME",
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

Sources can be any folder - documentation, notes, project files, etc.

## rclone Behavior

```bash
rclone sync SOURCE DEST --checksum --delete-during
```

- **Mirror mode**: Destination matches source exactly
- **Deletions sync**: Files removed from source are removed from destination
- **Checksum**: File integrity verification

## Common Tasks

### Add new source
1. Edit `config/machine-configs/{HOSTNAME}.json`
2. Add entry to `sources` array (any folder path)
3. Test: `bun run syncr --dry-run`

### Modify rclone options
Edit `buildRcloneArgs()` function in `src/rclone.ts`

### Change filter rules
Edit `config/filters.txt` - uses rclone filter syntax

### Modify schedule
Edit `cron_restart` in `ecosystem.config.js`

## Testing

```bash
# Check tools
bun run syncr --check-tools

# List configured sources
bun run syncr --list-only

# Dry run (no changes)
bun run syncr --dry-run --verbose

# Actual sync
bun run syncr
```

## Code Conventions

- TypeScript with strict mode
- Bun runtime
- Use `logger.info()`, `logger.error()`, etc. for logging
- Errors: `process.exit(1)` with ERROR log
- Warnings: continue processing, log WARN
- Cross-platform paths via `node:path` and `node:os`

## File Locations

```
syncr/
+-- src/                 # TypeScript source
+-- config/              # Configuration files
+-- logs/                # Daily log files
+-- backups/             # Synced data
+-- docs/                # Documentation
+-- package.json
+-- ecosystem.config.js  # pm2 config
```

## Dependencies

- **Bun**: Runtime - https://bun.sh
- **rclone**: Sync engine - https://rclone.org/downloads/
- **pm2**: Scheduling (optional) - `npm install -g pm2`

## Quick Validation

```bash
# Verify setup
bun run syncr --check-tools
bun run syncr --list-only
bun run syncr --dry-run

# Check pm2 status
pm2 status
```

## Building

```bash
# Development
bun install
bun run syncr --help

# Single binary
bun build src/cli.ts --compile --outfile dist/syncr
```
