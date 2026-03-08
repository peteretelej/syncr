// Package sync wraps rclone's bisync functionality for bidirectional folder sync.
package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// Import local backend to register it
	_ "github.com/rclone/rclone/backend/local"

	"github.com/rclone/rclone/cmd/bisync"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/filter"
)

// ResyncMode determines which path takes priority during resync.
type ResyncMode int

const (
	// ResyncNone means no resync - normal sync operation.
	ResyncNone ResyncMode = iota
	// ResyncPath1 means local (path1) wins during resync.
	ResyncPath1
	// ResyncPath2 means cloud (path2) wins during resync.
	ResyncPath2
)

// BisyncOptions configures a bisync operation.
type BisyncOptions struct {
	// Resync triggers a full resync (required for first-time init or recovery).
	Resync bool
	// ResyncMode determines which path wins during resync.
	ResyncMode ResyncMode
	// DryRun shows changes without applying them.
	DryRun bool
	// Verbose enables detailed logging.
	Verbose bool
	// SyncrDataDir is the path to the _syncr folder for working files.
	SyncrDataDir string
	// Excludes is a list of glob patterns to exclude from sync.
	Excludes []string
	// BackupDir1 is the directory for backing up changed/deleted files from path1 (local).
	BackupDir1 string
	// BackupDir2 is the directory for backing up changed/deleted files from path2 (cloud).
	BackupDir2 string
	// ConflictResolve is the conflict resolution strategy (e.g. "newer", "older", "path1").
	ConflictResolve string
	// ConflictSuffix is the custom suffix for conflict files (e.g. "{DateOnly}").
	ConflictSuffix string
}

// BisyncResult contains the results of a bisync operation.
type BisyncResult struct {
	// Duration of the sync operation.
	Duration time.Duration
	// Success indicates if the sync completed without errors.
	Success bool
	// Error message if sync failed.
	Error string
}

var initialized bool

// Init initializes the rclone library. Must be called before RunBisync.
func Init() error {
	if initialized {
		return nil
	}

	// Use in-memory config only - no rclone.conf needed for local-to-local sync
	config.SetConfigPath("")
	configfile.Install()

	initialized = true
	return nil
}

// RunBisync performs a bidirectional sync between localPath and cloudPath.
func RunBisync(ctx context.Context, localPath, cloudPath string, opts BisyncOptions) (*BisyncResult, error) {
	start := time.Now()
	result := &BisyncResult{}

	// Ensure rclone is initialized
	if err := Init(); err != nil {
		result.Error = fmt.Sprintf("init failed: %v", err)
		return result, err
	}

	// Validate paths
	if err := validatePath(localPath); err != nil {
		result.Error = fmt.Sprintf("local path invalid: %v", err)
		return result, fmt.Errorf("local path: %w", err)
	}
	if err := validatePath(cloudPath); err != nil {
		result.Error = fmt.Sprintf("cloud path invalid: %v", err)
		return result, fmt.Errorf("cloud path: %w", err)
	}

	// Create bisync working directory
	workdir := filepath.Join(opts.SyncrDataDir, "bisync")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		result.Error = fmt.Sprintf("failed to create workdir: %v", err)
		return result, fmt.Errorf("create workdir: %w", err)
	}

	// Apply exclude filters before creating filesystem objects
	if len(opts.Excludes) > 0 {
		var fi *filter.Filter
		ctx, fi = filter.AddConfig(ctx)
		for _, pattern := range opts.Excludes {
			if err := fi.Add(false, pattern); err != nil {
				result.Error = fmt.Sprintf("invalid exclude pattern %q: %v", pattern, err)
				return result, fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
			}
		}
	}

	// Create filesystem objects
	localFs, err := fs.NewFs(ctx, localPath)
	if err != nil {
		result.Error = fmt.Sprintf("local fs failed: %v", err)
		return result, fmt.Errorf("create local fs: %w", err)
	}

	cloudFs, err := fs.NewFs(ctx, cloudPath)
	if err != nil {
		result.Error = fmt.Sprintf("cloud fs failed: %v", err)
		return result, fmt.Errorf("create cloud fs: %w", err)
	}

	// Configure bisync options
	bisyncOpts, err := buildBisyncOpts(opts, workdir)
	if err != nil {
		result.Error = fmt.Sprintf("bisync options: %v", err)
		return result, err
	}

	// Run bisync
	err = bisync.Bisync(ctx, localFs, cloudFs, bisyncOpts)
	result.Duration = time.Since(start)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}

	result.Success = true
	return result, nil
}

// buildBisyncOpts constructs bisync.Options from BisyncOptions and a working directory.
func buildBisyncOpts(opts BisyncOptions, workdir string) (*bisync.Options, error) {
	bisyncOpts := &bisync.Options{
		Workdir:     workdir,
		MaxDelete:   50, // Safety: abort if >50% would be deleted
		Resilient:   true,
		Recover:     true,
		DryRun:      opts.DryRun,
		CheckSync:   bisync.CheckSyncTrue,
		Compare:     bisync.CompareOpt{Size: true, Modtime: true},
		CompareFlag: "size,modtime",
		BackupDir1:  opts.BackupDir1,
		BackupDir2:  opts.BackupDir2,
	}

	// Configure resync if needed
	if opts.Resync {
		bisyncOpts.Resync = true
		switch opts.ResyncMode {
		case ResyncPath1:
			bisyncOpts.ResyncMode = bisync.PreferPath1
		case ResyncPath2:
			bisyncOpts.ResyncMode = bisync.PreferPath2
		default:
			// Default to keeping superset of both
			bisyncOpts.ResyncMode = bisync.PreferNone
		}
	}

	// Configure conflict resolution
	if opts.ConflictResolve != "" {
		if err := bisyncOpts.ConflictResolve.Set(opts.ConflictResolve); err != nil {
			return nil, fmt.Errorf("invalid conflict_resolve %q: %w", opts.ConflictResolve, err)
		}
	}
	if opts.ConflictSuffix != "" {
		bisyncOpts.ConflictSuffixFlag = opts.ConflictSuffix
	}

	return bisyncOpts, nil
}

// validatePath checks that a path exists and is a directory.
func validatePath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", path)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	return nil
}
