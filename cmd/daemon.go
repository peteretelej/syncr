package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/discover"
	"github.com/peteretelej/syncr/internal/logger"
	"github.com/peteretelej/syncr/internal/state"
	"github.com/peteretelej/syncr/internal/sync"
)

// Daemon runs the continuous sync loop.
func Daemon(configPath string, verbose, dryRun bool) {
	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid config: %v\n", err)
		os.Exit(1)
	}

	// Create logger
	log, err := logger.New(cfg.SyncrDataDir(), verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	// Load state
	st, err := state.Load(cfg.SyncrDataDir())
	if err != nil {
		log.Error("Failed to load state: %v", err)
		os.Exit(1)
	}

	// Write PID file
	pidFile := filepath.Join(cfg.SyncrDataDir(), "syncr.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		log.Warn("Failed to write PID file: %v", err)
	}
	defer os.Remove(pidFile)

	// Calculate sync interval
	interval := time.Duration(cfg.SyncIntervalMinutes) * time.Minute
	oldInterval := cfg.SyncIntervalMinutes

	// Track config file modification time for reload detection
	resolvedConfigPath := cfg.Path()
	lastModTime, _ := configModTime(resolvedConfigPath)

	// Count enabled projects
	enabledCount := 0
	for _, p := range cfg.Projects {
		if p.Enabled {
			enabledCount++
		}
	}

	log.Info("Starting syncr daemon")
	log.Info("Sync interval: %v", interval)
	log.Info("Projects: %d enabled", enabledCount)

	// Set up signal handling with cancellable context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create ticker
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Track which projects have already warned about being skipped
	warnedSkips := make(map[string]bool)

	// Run initial sync
	log.Info("Running initial sync (%d projects)...", enabledCount)
	runDaemonSync(ctx, cfg, st, log, warnedSkips, verbose, dryRun)

	// Main loop
	for {
		select {
		case <-ticker.C:
			// Check for config changes
			cfg, lastModTime = maybeReloadConfig(resolvedConfigPath, cfg, lastModTime, log)

			// Reset ticker if interval changed
			if cfg.SyncIntervalMinutes != oldInterval {
				ticker.Reset(time.Duration(cfg.SyncIntervalMinutes) * time.Minute)
				log.Info("Sync interval changed: %v -> %v",
					time.Duration(oldInterval)*time.Minute,
					time.Duration(cfg.SyncIntervalMinutes)*time.Minute)
				oldInterval = cfg.SyncIntervalMinutes
			}

			// Count enabled projects (config may have been reloaded)
			scheduledEnabled := 0
			for _, p := range cfg.Projects {
				if p.Enabled {
					scheduledEnabled++
				}
			}
			log.Info("Running scheduled sync (%d projects)...", scheduledEnabled)
			runDaemonSync(ctx, cfg, st, log, warnedSkips, verbose, dryRun)

		case <-ctx.Done():
			log.Info("Received shutdown signal, shutting down...")
			return
		}
	}
}

// configModTime returns the modification time of the given file path.
func configModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// maybeReloadConfig checks if the config file has been modified and reloads it if so.
// Returns the config to use (new or current) and the updated mod time.
// If the config file is inaccessible or the new config is invalid, the current config is kept.
func maybeReloadConfig(configPath string, current *config.Config, lastModTime time.Time, log *logger.Logger) (*config.Config, time.Time) {
	modTime, err := configModTime(configPath)
	if err != nil {
		log.Warn("Cannot stat config file (keeping previous config): %v", err)
		return current, lastModTime
	}

	if modTime.Equal(lastModTime) {
		return current, lastModTime
	}

	newCfg, err := config.Load(configPath)
	if err != nil {
		log.Error("Config reload failed (keeping previous config): %v", err)
		return current, modTime
	}

	if err := newCfg.Validate(); err != nil {
		log.Error("Config reload failed (keeping previous config): %v", err)
		return current, modTime
	}

	enabledCount := 0
	for _, p := range newCfg.Projects {
		if p.Enabled {
			enabledCount++
		}
	}
	log.Info("Config reloaded: %d projects (%d enabled)", len(newCfg.Projects), enabledCount)

	return newCfg, modTime
}

// MaxConsecutiveErrors is the threshold after which we suggest re-initialization.
const MaxConsecutiveErrors = 5

// runDaemonSync syncs all enabled, initialized projects.
func runDaemonSync(ctx context.Context, cfg *config.Config, st *state.State, log *logger.Logger, warnedSkips map[string]bool, verbose, dryRun bool) {
	if _, err := runScheduledDiscovery(cfg, st, verbose, dryRun, log); err != nil {
		log.Warn("Folder discovery failed: %v", err)
	}

	missingDiscovered := make(map[string]bool)
	for _, project := range discover.MissingDiscovered(cfg) {
		missingDiscovered[project.Name] = true
	}

	for _, project := range cfg.Projects {
		if !project.Enabled {
			continue
		}
		if missingDiscovered[project.Name] {
			log.Warn("%s: discovered local path missing, skipped: %s", project.Name, project.LocalPath)
			continue
		}

		if !st.IsInitialized(project.Name) {
			if !warnedSkips[project.Name] {
				log.Info("%s: skipped (not initialized, run: syncr init %s)", project.Name, project.Name)
				warnedSkips[project.Name] = true
			} else {
				log.Debug("%s: skipped (not initialized)", project.Name)
			}
			continue
		}

		// Check if project has too many consecutive errors
		ps := st.GetProject(project.Name)
		if ps.ErrorCount >= MaxConsecutiveErrors {
			log.Warn("%s: skipped - %d consecutive errors (run: syncr init %s --force)", project.Name, ps.ErrorCount, project.Name)
			continue
		}

		syncPath := filepath.Join(cfg.SyncRoot, project.SyncPath)

		// Check paths exist with actionable error messages
		if !pathExists(project.LocalPath) {
			log.Warn("%s: local path missing: %s", project.Name, project.LocalPath)
			log.Warn("  Fix: Ensure the directory exists or update config")
			if !dryRun {
				st.RecordError(project.Name, fmt.Errorf("local path missing: %s", project.LocalPath))
			}
			continue
		}

		if !pathExists(syncPath) {
			log.Warn("%s: sync folder missing: %s", project.Name, syncPath)
			log.Warn("  Fix: Run 'syncr init %s --force' to recreate sync folder from local files", project.Name)
			if !dryRun {
				st.RecordError(project.Name, fmt.Errorf("sync folder missing: %s", syncPath))
			}
			continue
		}

		excludes := cfg.ResolvedExcludesFor(&project)

		// Take pre-sync snapshots for change detection
		var preLocalSnap, preSyncSnap sync.DirSnapshot
		var snapErr bool
		preLocalSnap, snapE := sync.TakeSnapshot(project.LocalPath, excludes...)
		if snapE != nil {
			log.Debug("%s: snapshot warning (local): %v", project.Name, snapE)
			snapErr = true
		}
		preSyncSnap, snapE = sync.TakeSnapshot(syncPath, excludes...)
		if snapE != nil {
			log.Debug("%s: snapshot warning (sync folder): %v", project.Name, snapE)
			snapErr = true
		}
		preConflicts, _ := sync.CountConflicts(syncPath, cfg.ResolvedConflictSuffix(), excludes...)

		start := time.Now()

		opts := sync.BisyncOptions{
			Resync:          false,
			DryRun:          dryRun,
			Verbose:         false,
			SyncrDataDir:    cfg.SyncrDataDir(),
			Excludes:        excludes,
			ConflictResolve: cfg.ResolvedConflictResolve(project.Name),
			ConflictSuffix:  cfg.ResolvedConflictSuffix(),
		}

		if project.BackupDir {
			timestamp := sync.TrashTimestamp()
			trashPath := filepath.Join(cfg.TrashDir(project.Name), timestamp)
			opts.BackupDir1 = trashPath
			opts.BackupDir2 = trashPath
			if !dryRun && cfg.BackupRetentionDays() > 0 {
				deleted, err := sync.CleanTrash(cfg.TrashDir(project.Name), cfg.BackupRetentionDays())
				if err != nil {
					log.Debug("%s: trash cleanup error: %v", project.Name, err)
				} else if deleted > 0 {
					log.Debug("%s: cleaned %d old trash directories", project.Name, deleted)
				}
			}
		}

		_, err := sync.RunBisync(ctx, project.LocalPath, syncPath, opts)
		duration := time.Since(start)

		if err != nil {
			log.Error("%s: sync failed (%v)", project.Name, err)
			if !dryRun {
				st.RecordError(project.Name, err)
			}

			// Check if this pushes us over the threshold
			newErrorCount := st.GetProject(project.Name).ErrorCount
			if newErrorCount >= MaxConsecutiveErrors {
				log.Warn("  Project has %d consecutive errors", newErrorCount)
				log.Warn("  Suggestion: Run 'syncr init %s --force' to resync (local files are preserved)", project.Name)
			}
			continue
		}

		// Check for conflicts
		conflictCount, _ := sync.CountConflicts(syncPath, cfg.ResolvedConflictSuffix(), excludes...)

		if conflictCount > 0 {
			log.Warn("%s: synced with %d conflict(s) (%v)", project.Name, conflictCount, duration.Round(time.Millisecond))
			if !dryRun {
				st.RecordConflicts(project.Name, conflictCount)
			}
		} else {
			log.Info("%s: synced (%v)", project.Name, duration.Round(time.Millisecond))
			if !dryRun {
				st.RecordSuccess(project.Name)
			}
		}

		// Fire hooks after successful sync
		if !dryRun && !snapErr && project.Hooks != nil {
			hookTimeout := 30 * time.Second
			if project.HookTimeoutSeconds > 0 {
				hookTimeout = time.Duration(project.HookTimeoutSeconds) * time.Second
			}

			postLocalSnap, errL := sync.TakeSnapshot(project.LocalPath, excludes...)
			postSyncSnap, errS := sync.TakeSnapshot(syncPath, excludes...)
			if errL != nil || errS != nil {
				log.Debug("%s: snapshot warning (post-sync): skipping hooks", project.Name)
			} else {
				filesChanged := preLocalSnap.Changed(postLocalSnap) || preSyncSnap.Changed(postSyncSnap)
				newConflicts := conflictCount - preConflicts
				if newConflicts < 0 {
					newConflicts = 0
				}

				hookEnv := sync.HookEnv{
					ProjectName:  project.Name,
					LocalPath:    project.LocalPath,
					SyncPath:     syncPath,
					FilesChanged: boolInt(filesChanged),
					Conflicts:    newConflicts,
				}

				// post_sync hook
				if project.Hooks.PostSync != "" && filesChanged {
					out, hookErr := sync.RunHook(ctx, project.Hooks.PostSync, project.LocalPath, hookEnv, hookTimeout)
					if hookErr != nil {
						log.Warn("%s: post_sync hook error: %v", project.Name, hookErr)
					} else if out != "" {
						log.Debug("%s: post_sync: %s", project.Name, out)
					}
				}

				// on_conflict hook
				if project.Hooks.OnConflict != "" && newConflicts > 0 {
					out, hookErr := sync.RunHook(ctx, project.Hooks.OnConflict, project.LocalPath, hookEnv, hookTimeout)
					if hookErr != nil {
						log.Warn("%s: on_conflict hook error: %v", project.Name, hookErr)
					} else if out != "" {
						log.Debug("%s: on_conflict: %s", project.Name, out)
					}
				}
			}
		}
	}

	// Save state after each sync cycle
	if !dryRun {
		if err := st.Save(); err != nil {
			log.Error("Failed to save state: %v", err)
		}
	}
}
