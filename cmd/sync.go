package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/peteretelej/syncr/internal/config"
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

	if len(args) > 0 {
		fmt.Printf("Syncing project: %s\n", args[0])
	} else {
		fmt.Printf("Syncing %d enabled project(s)...\n", len(projectsToSync))
	}

	if dryRun {
		fmt.Println("[DRY-RUN mode enabled]")
	}
	fmt.Println()

	// Sync each project
	var successCount, failCount, skippedCount int
	ctx := context.Background()

	for _, project := range projectsToSync {
		result := syncProject(ctx, cfg, st, project, verbose, dryRun)
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
func syncProject(ctx context.Context, cfg *config.Config, st *state.State, project *config.Project, verbose, dryRun bool) string {
	// Check if initialized
	if !st.IsInitialized(project.Name) {
		fmt.Printf("  %s: skipped (not initialized, run: syncr init %s)\n", project.Name, project.Name)
		return "skipped"
	}

	// Check for too many consecutive errors
	ps := st.GetProject(project.Name)
	if ps.ErrorCount >= MaxConsecutiveErrors {
		fmt.Printf("  %s: skipped (%d consecutive errors)\n", project.Name, ps.ErrorCount)
		fmt.Printf("    Fix: Run 'syncr init %s --force' to re-initialize\n", project.Name)
		return "skipped"
	}

	syncPath := filepath.Join(cfg.SyncRoot, project.SyncPath)

	// Check paths exist with actionable error messages
	if !pathExists(project.LocalPath) {
		fmt.Printf("  %s: failed (local path missing: %s)\n", project.Name, project.LocalPath)
		fmt.Printf("    Fix: Ensure the directory exists or update config\n")
		st.RecordError(project.Name, fmt.Errorf("local path missing: %s", project.LocalPath))
		return "failed"
	}

	if !pathExists(syncPath) {
		fmt.Printf("  %s: failed (cloud path missing: %s)\n", project.Name, syncPath)
		fmt.Printf("    Fix: Run 'syncr init %s' to create it\n", project.Name)
		st.RecordError(project.Name, fmt.Errorf("cloud path missing: %s", syncPath))
		return "failed"
	}

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
		fmt.Printf("  %s: failed (%v)\n", project.Name, err)
		if !dryRun {
			st.RecordError(project.Name, err)

			// Check if this pushes us over the error threshold
			newErrorCount := st.GetProject(project.Name).ErrorCount
			if newErrorCount >= MaxConsecutiveErrors {
				fmt.Printf("    Warning: %d consecutive errors\n", newErrorCount)
				fmt.Printf("    Suggestion: Run 'syncr init %s --force' to re-initialize\n", project.Name)
			}
		}
		return "failed"
	}

	// Check for conflicts
	conflictCount, _ := sync.CountConflicts(syncPath)

	if conflictCount > 0 {
		fmt.Printf("  %s: synced with %d conflict(s) (%v)\n", project.Name, conflictCount, duration.Round(time.Millisecond))
		if !dryRun {
			st.RecordConflicts(project.Name, conflictCount)
		}
	} else {
		fmt.Printf("  %s: synced (%v)\n", project.Name, duration.Round(time.Millisecond))
		if !dryRun {
			st.RecordSuccess(project.Name)
		}
	}

	_ = result // result contains additional info if needed
	return "success"
}
