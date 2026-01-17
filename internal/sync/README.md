# internal/sync - rclone Bisync Integration

This package wraps rclone's bisync functionality for syncr's bidirectional sync needs.

## API Investigation Findings

### Entry Point

```go
import "github.com/rclone/rclone/cmd/bisync"

// Main bisync function
err := bisync.Bisync(ctx, fs1, fs2, opts)
```

### Creating Filesystem Objects

```go
import "github.com/rclone/rclone/fs"

// For local paths, just pass the path
localFs, err := fs.NewFs(ctx, "/path/to/local/folder")
cloudFs, err := fs.NewFs(ctx, "/path/to/cloud/folder")
```

### Options Struct

The `bisync.Options` struct controls bisync behavior:

```go
type Options struct {
    Resync                bool          // Trigger resync (first-time init)
    ResyncMode            Prefer        // path1, path2, newer, older, etc.
    DryRun                bool          // Show changes without applying
    Workdir               string        // Working directory for .lst files
    MaxDelete             int           // Max deletion percentage (0-100)
    Resilient             bool          // Allow retry after certain errors
    Recover               bool          // Recover from interruptions
    Compare               CompareOpt    // Compare options
    CompareFlag           string        // "size,modtime,checksum"
    ConflictResolve       Prefer        // Conflict resolution strategy
    ConflictLoser         ConflictLoserAction
    ConflictSuffixFlag    string        // Suffix for conflict files
    // ... more options
}
```

### Prefer Values (for ResyncMode and ConflictResolve)

```go
const (
    PreferNone    Prefer = iota  // No preference / don't resync
    PreferPath1                   // Prefer path1 (local)
    PreferPath2                   // Prefer path2 (cloud)
    PreferNewer                   // Prefer newer file
    PreferOlder                   // Prefer older file
    PreferLarger                  // Prefer larger file
    PreferSmaller                 // Prefer smaller file
)
```

### CompareOpt Fields

```go
type CompareOpt struct {
    Modtime  bool  // Compare modification times
    Size     bool  // Compare file sizes
    Checksum bool  // Compare checksums
    // ... hash types
}
```

### Conflict Loser Actions

```go
const (
    ConflictLoserNumber   // file.conflict1, file.conflict2
    ConflictLoserPathname // file.path1, file.path2
    ConflictLoserDelete   // Delete loser, keep winner only
)
```

## Key Behaviors

1. **Workdir**: Bisync stores state files (`.lst` files) in the workdir. These track file listings between runs.

2. **Resync**: Required on first run or after errors. Creates fresh baseline listings.

3. **ResyncMode**: During resync, determines which path "wins" if both have files:
   - `PreferPath1`: Local files win (push to cloud)
   - `PreferPath2`: Cloud files win (pull from cloud)

4. **MaxDelete**: Safety check - aborts if deletion exceeds percentage (default 50%)

5. **Conflict Handling**: When both sides change the same file:
   - Creates `.conflict1`, `.conflict2` suffixed files
   - Or uses `ConflictResolve` strategy if set

## Usage in Syncr

```go
// Initialize filesystem objects
localFs, _ := fs.NewFs(ctx, localPath)
cloudFs, _ := fs.NewFs(ctx, cloudPath)

// Configure options
opts := &bisync.Options{
    Workdir:    filepath.Join(syncrDataDir, "bisync"),
    MaxDelete:  50,
    Resilient:  true,
    Recover:    true,
    Compare:    bisync.CompareOpt{Size: true, Modtime: true, Checksum: true},
}

// For first-time init
if needsInit {
    opts.Resync = true
    opts.ResyncMode = bisync.PreferPath2 // or PreferPath1
}

// Run bisync
err := bisync.Bisync(ctx, localFs, cloudFs, opts)
```
