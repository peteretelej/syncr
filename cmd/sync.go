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
	st, err := state.LoadLocal()
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

	start := time.Now()

	opts := sync.BisyncOptions{
		Resync:       false,
		DryRun:       dryRun,
		Verbose:      verbose,
		SyncrDataDir: cfg.SyncrDataDir(),
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
	conflictCount, _ := sync.CountConflicts(syncPath)

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

	_ = result // result contains additional info if needed
	return "success"
}
