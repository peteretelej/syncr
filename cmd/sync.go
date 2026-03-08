package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/progress"
	"github.com/peteretelej/syncr/internal/state"
	"github.com/peteretelej/syncr/internal/sync"
)

// Sync runs a one-shot sync for a project or all projects.
func Sync(args []string, configPath string, verbose, dryRun bool) {
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

	// Load state
	st, err := state.Load(cfg.SyncrDataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	// Determine which projects to sync
	var projectsToSync []*config.Project
	if len(args) > 0 {
		// Specific project
		project := cfg.GetProject(args[0])
		if project == nil {
			fmt.Fprintf(os.Stderr, "Error: project %q not found in config\n", args[0])
			os.Exit(1)
		}
		projectsToSync = []*config.Project{project}
	} else {
		// All enabled projects
		for i := range cfg.Projects {
			if cfg.Projects[i].Enabled {
				projectsToSync = append(projectsToSync, &cfg.Projects[i])
			}
		}
	}

	if len(projectsToSync) == 0 {
		fmt.Println("No enabled projects to sync.")
		return
	}

	// Sync each project
	prog := progress.New(os.Stdout, dryRun, verbose)
	var successCount, failCount, skippedCount int
	ctx := context.Background()

	for _, project := range projectsToSync {
		result := syncProject(ctx, cfg, st, project, verbose, dryRun, prog)
		switch result {
		case "success":
			successCount++
		case "failed":
			failCount++
		case "skipped":
			skippedCount++
		}
	}

	// Save state
	if !dryRun {
		if err := st.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
		}
	}

	// Print summary
	fmt.Println()
	fmt.Printf("Summary: %d synced", successCount)
	if failCount > 0 {
		fmt.Printf(", %d failed", failCount)
	}
	if skippedCount > 0 {
		fmt.Printf(", %d skipped", skippedCount)
	}
	fmt.Println()

	if failCount > 0 {
		os.Exit(1)
	}
}

// syncProject syncs a single project and returns "success", "failed", or "skipped".
func syncProject(ctx context.Context, cfg *config.Config, st *state.State, project *config.Project, verbose, dryRun bool, prog *progress.Progress) string {
	// Check if initialized
	if !st.IsInitialized(project.Name) {
		prog.Skip(project.Name, fmt.Sprintf("not initialized, run: syncr init %s", project.Name))
		return "skipped"
	}

	// Check for too many consecutive errors
	ps := st.GetProject(project.Name)
	if ps.ErrorCount >= MaxConsecutiveErrors {
		prog.Skip(project.Name, fmt.Sprintf("%d consecutive errors", ps.ErrorCount))
		prog.Hint("Fix: Run 'syncr init %s --force' to resync (local files are preserved)", project.Name)
		return "skipped"
	}

	syncPath := filepath.Join(cfg.SyncRoot, project.SyncPath)

	// Check paths exist with actionable error messages
	if !pathExists(project.LocalPath) {
		prog.Start(project.Name)
		prog.Fail(fmt.Errorf("local path missing: %s", project.LocalPath), project.LocalPath, syncPath)
		prog.Hint("Fix: Ensure the directory exists or update config")
		st.RecordError(project.Name, fmt.Errorf("local path missing: %s", project.LocalPath))
		return "failed"
	}

	if !pathExists(syncPath) {
		prog.Start(project.Name)
		prog.Fail(fmt.Errorf("sync folder missing: %s", syncPath), project.LocalPath, syncPath)
		prog.Hint("Fix: Run 'syncr init %s --force' to recreate sync folder from local files", project.Name)
		st.RecordError(project.Name, fmt.Errorf("sync folder missing: %s", syncPath))
		return "failed"
	}

	prog.Start(project.Name)

	// Show excluded patterns during dry-run
	if dryRun && len(project.Exclude) > 0 {
		fmt.Println("Excluded patterns:")
		for _, pattern := range project.Exclude {
			fmt.Printf("  %s\n", pattern)
		}
	}

	// Show configured hooks during dry-run
	if dryRun && project.Hooks != nil {
		if project.Hooks.PostSync != "" {
			fmt.Printf("Would run post_sync hook: %s\n", project.Hooks.PostSync)
		}
		if project.Hooks.OnConflict != "" {
			fmt.Printf("Would run on_conflict hook: %s\n", project.Hooks.OnConflict)
		}
	}

	// Take pre-sync snapshots for change detection
	var preLocalSnap, preSyncSnap sync.DirSnapshot
	var snapErr bool
	var snapE error
	preLocalSnap, snapE = sync.TakeSnapshot(project.LocalPath)
	if snapE != nil {
		prog.Detail("snapshot warning (local): %v", snapE)
		snapErr = true
	}
	preSyncSnap, snapE = sync.TakeSnapshot(syncPath)
	if snapE != nil {
		prog.Detail("snapshot warning (sync folder): %v", snapE)
		snapErr = true
	}
	preConflicts, _ := sync.CountConflicts(syncPath, cfg.ResolvedConflictSuffix())

	start := time.Now()

	// Show conflict settings during dry-run
	if dryRun {
		resolve := cfg.ResolvedConflictResolve(project.Name)
		if resolve != "" && resolve != "none" {
			fmt.Printf("Would resolve conflicts: %s\n", resolve)
		}
		suffix := cfg.ResolvedConflictSuffix()
		if suffix != "" {
			fmt.Printf("Conflict suffix: %s\n", suffix)
		}
	}

	opts := sync.BisyncOptions{
		Resync:          false,
		DryRun:          dryRun,
		Verbose:         verbose,
		SyncrDataDir:    cfg.SyncrDataDir(),
		Excludes:        project.Exclude,
		ConflictResolve: cfg.ResolvedConflictResolve(project.Name),
		ConflictSuffix:  cfg.ResolvedConflictSuffix(),
	}

	if project.BackupDir {
		timestamp := sync.TrashTimestamp()
		trashPath := filepath.Join(cfg.TrashDir(project.Name), timestamp)
		opts.BackupDir1 = trashPath
		opts.BackupDir2 = trashPath
		if dryRun {
			fmt.Printf("Would set backup-dir: %s\n", trashPath)
		} else if cfg.BackupRetentionDays() > 0 {
			deleted, err := sync.CleanTrash(cfg.TrashDir(project.Name), cfg.BackupRetentionDays())
			if err != nil {
				prog.Detail("trash cleanup error: %v", err)
			} else if deleted > 0 {
				prog.Detail("cleaned %d old trash directories", deleted)
			}
		}
	}

	result, err := sync.RunBisync(ctx, project.LocalPath, syncPath, opts)
	duration := time.Since(start)

	if err != nil {
		friendly, raw := sync.FriendlyError(err)
		prog.Fail(fmt.Errorf("%s", friendly), project.LocalPath, syncPath)
		prog.Detail("rclone: %s", raw)
		if !dryRun {
			st.RecordError(project.Name, err)

			// Check if this pushes us over the error threshold
			newErrorCount := st.GetProject(project.Name).ErrorCount
			if newErrorCount >= MaxConsecutiveErrors {
				prog.Hint("Warning: %d consecutive errors", newErrorCount)
				prog.Hint("Suggestion: Run 'syncr init %s --force' to resync (local files are preserved)", project.Name)
			}
		}
		return "failed"
	}

	// Check for conflicts
	conflictCount, _ := sync.CountConflicts(syncPath, cfg.ResolvedConflictSuffix())

	if conflictCount > 0 {
		prog.Done(duration, conflictCount)
		if !dryRun {
			st.RecordConflicts(project.Name, conflictCount)
		}
	} else {
		prog.Done(duration, 0)
		if !dryRun {
			st.RecordSuccess(project.Name)
		}
	}

	// Fire hooks after successful sync (not in dry-run)
	if !dryRun && !snapErr && project.Hooks != nil {
		hookTimeout := 30 * time.Second
		if project.HookTimeoutSeconds > 0 {
			hookTimeout = time.Duration(project.HookTimeoutSeconds) * time.Second
		}

		// Detect changes by comparing snapshots
		postLocalSnap, errL := sync.TakeSnapshot(project.LocalPath)
		postSyncSnap, errS := sync.TakeSnapshot(syncPath)
		if errL != nil || errS != nil {
			prog.Detail("snapshot warning (post-sync): skipping hooks")
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
					prog.Hint("post_sync hook error: %v", hookErr)
				} else if out != "" {
					prog.Detail("post_sync: %s", out)
				}
			}

			// on_conflict hook
			if project.Hooks.OnConflict != "" && newConflicts > 0 {
				out, hookErr := sync.RunHook(ctx, project.Hooks.OnConflict, project.LocalPath, hookEnv, hookTimeout)
				if hookErr != nil {
					prog.Hint("on_conflict hook error: %v", hookErr)
				} else if out != "" {
					prog.Detail("on_conflict: %s", out)
				}
			}
		}
	}

	_ = result // result contains additional info if needed
	return "success"
}

// boolInt returns 1 if b is true, 0 otherwise.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
