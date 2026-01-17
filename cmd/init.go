package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/state"
	"github.com/peteretelej/syncr/internal/sync"
)

// Init initializes a project for first-time sync.
func Init(args []string, configPath string, verbose, dryRun bool) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: project name required")
		fmt.Fprintln(os.Stderr, "Usage: syncr init <project>")
		os.Exit(1)
	}

	projectName := args[0]
	force := false
	if len(args) > 1 && args[1] == "--force" {
		force = true
	}

	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Find project
	project := cfg.GetProject(projectName)
	if project == nil {
		fmt.Fprintf(os.Stderr, "Error: project %q not found in config\n", projectName)
		os.Exit(1)
	}

	// Load state
	st, err := state.Load(cfg.SyncrDataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	// Check if already initialized
	if st.IsInitialized(projectName) && !force {
		fmt.Printf("Project %q is already initialized.\n", projectName)
		fmt.Println("Use --force to re-initialize.")
		return
	}

	// Build paths
	cloudPath := filepath.Join(cfg.CloudRoot, project.CloudSubpath)

	fmt.Printf("Initializing project: %s\n", projectName)
	fmt.Printf("  Local:  %s\n", project.LocalPath)
	fmt.Printf("  Cloud:  %s\n", cloudPath)
	fmt.Println()

	// Check paths exist, create cloud folder if needed
	fmt.Println("Checking paths...")

	localExists := pathExists(project.LocalPath)
	if !localExists {
		fmt.Fprintf(os.Stderr, "Error: local path does not exist: %s\n", project.LocalPath)
		os.Exit(1)
	}

	// Create cloud folder if it doesn't exist
	if !pathExists(cloudPath) {
		if dryRun {
			fmt.Printf("  [DRY-RUN] Would create cloud folder: %s\n", cloudPath)
		} else {
			if err := os.MkdirAll(cloudPath, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating cloud folder: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("  Created cloud folder: %s\n", cloudPath)
		}
	}

	// Count files in each location
	localCount := countFiles(project.LocalPath)
	cloudCount := countFiles(cloudPath)

	fmt.Printf("  Local folder:  %d files\n", localCount)
	fmt.Printf("  Cloud folder:  %d files\n", cloudCount)
	fmt.Println()

	// Determine resync mode based on folder contents
	var resyncMode sync.ResyncMode
	var actionDesc string

	switch {
	case localCount == 0 && cloudCount > 0:
		// Cloud has files, local is empty - pull from cloud
		resyncMode = sync.ResyncPath2
		actionDesc = "Cloud has files, local is empty. Pulling from cloud..."
	case cloudCount == 0 && localCount > 0:
		// Local has files, cloud is empty - push to cloud
		resyncMode = sync.ResyncPath1
		actionDesc = "Local has files, cloud is empty. Pushing to cloud..."
	case localCount == 0 && cloudCount == 0:
		// Both empty - nothing to sync
		actionDesc = "Both folders are empty. Marking as initialized."
		if dryRun {
			fmt.Printf("[DRY-RUN] %s\n", actionDesc)
		} else {
			fmt.Println(actionDesc)
			st.MarkInitialized(projectName)
			if err := st.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("\nProject %q initialized successfully.\n", projectName)
		}
		return
	default:
		// Both have files - keep superset
		resyncMode = sync.ResyncNone
		actionDesc = "Both folders have files. Merging (keeping superset)..."
	}

	fmt.Println(actionDesc)

	if dryRun {
		fmt.Println()
		fmt.Println("[DRY-RUN] Would run bisync with --resync")
		fmt.Printf("[DRY-RUN] Would mark project %q as initialized\n", projectName)
		return
	}

	// Run bisync with resync
	fmt.Println()
	fmt.Println("Running bisync with --resync...")

	opts := sync.BisyncOptions{
		Resync:       true,
		ResyncMode:   resyncMode,
		DryRun:       false,
		Verbose:      verbose,
		SyncrDataDir: cfg.SyncrDataDir(),
	}

	ctx := context.Background()
	result, err := sync.RunBisync(ctx, project.LocalPath, cloudPath, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during sync: %v\n", err)
		st.RecordError(projectName, err)
		st.Save()
		os.Exit(1)
	}

	fmt.Printf("  Duration: %v\n", result.Duration.Round(1e8))

	// Mark as initialized
	st.MarkInitialized(projectName)
	st.RecordSuccess(projectName)
	if err := st.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nProject %q initialized successfully.\n", projectName)
}

// pathExists returns true if the path exists.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// countFiles returns the number of files (not directories) in a path.
func countFiles(path string) int {
	count := 0
	filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}
