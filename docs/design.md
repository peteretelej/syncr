# syncr - Technical Design

## Overview

Lightweight folder sync tool. Uses rclone for sync, Bun for runtime, pm2 for scheduling. Works well with cloud sync tools (OneDrive, Dropbox, Google Drive).

## Architecture

```
+---------------------+
|  pm2 (scheduler)    |
|  cron: 0 * * * *    |
+----------+----------+
           |
           v
+---------------------+
|  Bun Runtime        |
|  src/cli.ts         |
+----------+----------+
           |
           v
+---------------------+
|  Backup Module      |
|  src/backup.ts      |
+----------+----------+
           |
           +---> Config (JSON)
           |
           +---> rclone (sync engine)
           |
           +---> ./backups/HOSTNAME/
```

## Data Flow

```
Source Folders (any path)
    |
    +-> C:\Users\Peter\Projects\webapp\docs
    +-> C:\Users\Peter\Projects\api-server\_docs
    +-> D:\Notes\research
         |
         +---> [rclone sync --checksum --delete-during]
         |
         +---> ./backups/MACHINE-NAME/
                  |
                  +-> webapp-docs/
                  +-> api-server-docs/
                  +-> research/
```

## Directory Structure

```
syncr/
+-- src/
|   +-- cli.ts           # CLI entry point (Commander.js)
|   +-- backup.ts        # Main sync orchestration
|   +-- config.ts        # Config loading and path resolution
|   +-- rclone.ts        # rclone command builder and executor
|   +-- logger.ts        # Console and file logging
|   +-- types.ts         # TypeScript interfaces
+-- config/
|   +-- filters.txt      # rclone filter rules
|   +-- template.json    # Config template
|   +-- machine-configs/
|       +-- HOSTNAME.json  # Per-machine source list
+-- logs/
|   +-- syncr_YYYYMMDD.log  # Daily operation logs
+-- backups/
|   +-- MACHINE-NAME/
|       +-- PROJECT-NAME/    # Actual synced data
+-- package.json
+-- tsconfig.json
+-- ecosystem.config.js   # pm2 configuration
```

## Source Code Modules

### cli.ts

Entry point using Commander.js for argument parsing.

```typescript
// CLI flags
--dry-run      // Preview changes without copying
--verbose      // Show detailed output
--list-only    // List configured sources
--init         // Create initial config
--check-tools  // Verify rclone installed
```

### backup.ts

Main orchestration module:

1. Validates prerequisites (rclone, backup root)
2. Loads machine-specific configuration
3. Iterates through enabled sources
4. Executes rclone sync for each source
5. Logs results and summary

### config.ts

Configuration handling:

- Determines backup root path (current working directory)
- Loads and validates JSON config files
- Creates initial config with `--init` flag
- Resolves machine-specific config by hostname

### rclone.ts

rclone integration:

- Checks if rclone is installed
- Builds sync command with appropriate flags
- Executes rclone as child process
- Handles output and errors

Command construction:
```
rclone sync SOURCE DEST \
  --checksum \
  --delete-during \
  --filter-from filters.txt \
  [--dry-run] \
  [--verbose --progress]
```

### logger.ts

Logging with console and file output:

- Color-coded console output (INFO, SUCCESS, WARN, ERROR)
- Daily log files: `syncr_YYYYMMDD.log`
- Timestamp format: `[YYYY-MM-DD HH:MM:SS]`

### types.ts

TypeScript interfaces:

```typescript
interface BackupSource {
  name: string;
  path: string;
  enabled: boolean;
}

interface MachineConfig {
  machine_name: string;
  sources: BackupSource[];
}

interface BackupResult {
  source: BackupSource;
  success: boolean;
  error?: string;
  duration?: number;
}
```

## Configuration Schema

```json
{
  "machine_name": "HOSTNAME",
  "sources": [
    {
      "name": "ProjectName",
      "path": "C:\\Path\\To\\Folder",
      "enabled": true
    }
  ]
}
```

Fields:
- **machine_name**: Hostname identifier (auto-detected)
- **sources**: Array of backup sources
  - **name**: Project identifier (used in backup path)
  - **path**: Absolute path to folder
  - **enabled**: Boolean flag for active/inactive

## rclone Configuration

### Sync Options

```
rclone sync SOURCE DEST \
  --checksum           # Verify via checksums
  --delete-during      # Delete dest files not in source
  --filter-from FILE   # Apply exclusion rules
```

**Mirror Mode**: Destination becomes exact copy of source. Files deleted from source are deleted from destination.

### Filter Rules

Located in `config/filters.txt`:

```
# Exclude version control
- **/.git/**
- **/.svn/**

# Exclude dependencies
- **/node_modules/**
- **/__pycache__/**

# Exclude sensitive files
- **/token.txt
- **/tokens.txt
- **/*.token

# Exclude temp files
- *.tmp
- *.log

# Include everything else
+ **
```

## Scheduling with pm2

### ecosystem.config.js

```javascript
module.exports = {
  apps: [{
    name: 'syncr',
    script: 'src/cli.ts',
    interpreter: 'bun',
    cron_restart: '0 * * * *',  // Hourly
    autorestart: false,
    log_date_format: 'YYYY-MM-DD HH:mm:ss',
    error_file: 'logs/pm2-error.log',
    out_file: 'logs/pm2-out.log',
  }]
};
```

### pm2 Commands

```bash
pm2 start ecosystem.config.js   # Start
pm2 status                      # Check status
pm2 logs syncr                  # View logs
pm2 stop syncr                  # Stop
pm2 startup && pm2 save         # Persist across reboots
```

## Multi-Machine Strategy

### Isolation

Each machine syncs to its own subfolder:
```
backups/MACHINE1/ProjectA/
backups/MACHINE2/ProjectA/
```

Prevents:
- Cross-machine file conflicts
- Accidental overwrites
- Version inconsistencies

## Error Handling

| Error | Behavior |
|-------|----------|
| Missing source path | WARN log, continue processing |
| rclone failure | ERROR log, continue processing |
| Missing rclone | ERROR log, exit code 1 |
| Invalid config | ERROR log, exit code 1 |

Exit codes:
- 0: Success
- 1: Fatal error (missing tools, invalid config, sync failures)

## Logging

### Format

```
[YYYY-MM-DD HH:MM:SS] [LEVEL] Message
```

### Levels

- **INFO**: Normal operations
- **SUCCESS**: Successful completions
- **WARN**: Non-fatal issues
- **ERROR**: Fatal errors

### Files

- Console: Color-coded output
- File: `logs/syncr_YYYYMMDD.log`

## Performance

- Checksum-based change detection
- Only modified files transferred
- Parallel file operations (rclone default)
- Incremental sync in seconds to minutes

## Building

### Development

```bash
bun install
bun run syncr --help
```

### Single Binary

```bash
bun build src/cli.ts --compile --outfile dist/syncr
```

Creates standalone executable without runtime dependency.

## Dependencies

| Package | Purpose |
|---------|---------|
| commander | CLI argument parsing |
| @types/bun | Bun type definitions |
| @types/node | Node.js type definitions |

External:
- **rclone**: Sync engine (must be in PATH)
- **pm2**: Scheduling (optional)

## Extensibility

### Adding Sources

Edit machine config:
```json
{
  "name": "NewProject",
  "path": "/path/to/folder",
  "enabled": true
}
```

### Custom Filters

Edit `config/filters.txt`:
```
+ custom_pattern/**
- unwanted_pattern/**
```

### Custom Schedule

Edit `ecosystem.config.js`:
```javascript
cron_restart: '*/30 * * * *'  // Every 30 minutes
```

## Limitations

- No automatic conflict resolution
- No versioning (use cloud sync or git for that)
- No encryption (relies on filesystem/cloud provider)

## Recovery

### Restore Single Project

```bash
cp -r backups/MACHINE/PROJECT /target/path
```

### Cross-Machine Access

If using cloud sync, all machines have access to all backups:
```
backups/MACHINE1/  # Accessible on all machines
backups/MACHINE2/  # Accessible on all machines
```
