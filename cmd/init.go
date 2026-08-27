package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/state"
	"github.com/peteretelej/syncr/internal/sync"
)

// Init initializes a project for first-time sync.
func Init(args []string, configPath string, verbose, dryRun bool) {
	if len(args) == 0 {
		// No project specified - create starter config if none exists
		cfgPath := configPath
		if cfgPath == "" {
			cfgPath = os.Getenv("SYNCR_CONFIG")
		}
		if cfgPath == "" {
			cfgPath = "syncr.json"
		}
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			createStarterConfig(cfgPath)
			return
		}

		// Config exists: batch-init all uninitialized enabled projects
		batchInit(configPath, verbose, dryRun)
		return
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
		fmt.Println("Use --force to resync (local files are preserved).")
		return
	}

	if err := initProject(cfg, st, project, verbose, dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", color.RedString("Error: %v", err))
		os.Exit(1)
	}

	if !dryRun {
		if err := st.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			os.Exit(1)
		}
	}
}

// batchInit initializes all uninitialized enabled projects.
func batchInit(configPath string, verbose, dryRun bool) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	st, err := state.Load(cfg.SyncrDataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	// Collect enabled projects
	var enabled []*config.Project
	for i := range cfg.Projects {
		if cfg.Projects[i].Enabled {
			enabled = append(enabled, &cfg.Projects[i])
		}
	}

	if len(enabled) == 0 {
		fmt.Println("No enabled projects in config.")
		return
	}

	// Filter to uninitialized
	var uninitialized []*config.Project
	for _, p := range enabled {
		if !st.IsInitialized(p.Name) {
			uninitialized = append(uninitialized, p)
		}
	}

	if len(uninitialized) == 0 {
		fmt.Println("All enabled projects are already initialized.")
		return
	}

	fmt.Printf("Initializing %d uninitialized project(s)...\n", len(uninitialized))
	if dryRun {
		fmt.Println(color.CyanString("[DRY-RUN mode enabled]"))
	}
	fmt.Println()

	var failed []string
	successCount := 0

	for _, project := range uninitialized {
		err := initProject(cfg, st, project, verbose, dryRun)
		if err != nil {
			fmt.Printf("  %s\n", color.RedString("Error initializing %q: %v", project.Name, err))
			failed = append(failed, project.Name)
		} else {
			successCount++
		}
		fmt.Println()
	}

	// Save state once after all projects
	if !dryRun {
		if err := st.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
		}
	}

	// Print summary
	total := len(uninitialized)
	if successCount == total {
		fmt.Println(color.GreenString("Initialized %d of %d projects", successCount, total))
	} else {
		fmt.Println(color.YellowString("Initialized %d of %d projects", successCount, total))
	}
	if len(failed) > 0 {
		fmt.Printf("%s %s\n", color.RedString("Failed:"), strings.Join(failed, ", "))
		os.Exit(1)
	}
}

// initProject initializes a single project. Returns error instead of calling os.Exit.
// Does not call st.Save() - the caller is responsible for saving state.
func initProject(cfg *config.Config, st *state.State, project *config.Project, verbose, dryRun bool) error {
	dim := color.New(color.Faint)
	syncPath := filepath.Join(cfg.SyncRoot, project.SyncPath)

	fmt.Printf("Initializing project: %s\n", project.Name)
	dim.Printf("  Local:  %s\n", project.LocalPath)
	dim.Printf("  Sync folder:  %s\n", syncPath)
	fmt.Println()

	// Check paths exist, create sync folder if needed
	fmt.Println("Checking paths...")

	localExists := pathExists(project.LocalPath)
	if !localExists {
		return fmt.Errorf("local path does not exist: %s", project.LocalPath)
	}

	// Create sync folder if it doesn't exist
	if !pathExists(syncPath) {
		if dryRun {
			fmt.Printf("  %s Would create sync folder: %s\n", color.CyanString("[DRY-RUN]"), dim.Sprint(syncPath))
		} else {
			if err := os.MkdirAll(syncPath, 0755); err != nil {
				return fmt.Errorf("creating sync folder: %v", err)
			}
			fmt.Printf("  Created sync folder: %s\n", dim.Sprint(syncPath))
		}
	}

	// Count files in each location
	localCount := countFiles(project.LocalPath)
	syncCount := countFiles(syncPath)

	fmt.Printf("  Local folder:  %d files\n", localCount)
	fmt.Printf("  Sync folder:  %d files\n", syncCount)
	fmt.Println()

	// Determine resync mode based on folder contents
	var resyncMode sync.ResyncMode
	var actionDesc string

	switch {
	case localCount == 0 && syncCount > 0:
		resyncMode = sync.ResyncPath2
		actionDesc = "Sync folder has files, local is empty. Pulling from sync folder..."
	case syncCount == 0 && localCount > 0:
		resyncMode = sync.ResyncPath1
		actionDesc = "Local has files, sync folder is empty. Pushing to sync folder..."
	case localCount == 0 && syncCount == 0:
		actionDesc = "Both folders are empty. Marking as initialized."
		if dryRun {
			fmt.Printf("%s %s\n", color.CyanString("[DRY-RUN]"), actionDesc)
		} else {
			fmt.Println(actionDesc)
			st.MarkInitialized(project.Name)
			fmt.Println(color.GreenString("Project %q initialized successfully.", project.Name))
		}
		return nil
	default:
		resyncMode = sync.ResyncNone
		actionDesc = "Both folders have files. Merging (files from both sides will be kept)..."
	}

	fmt.Println(actionDesc)

	if dryRun {
		fmt.Println()
		fmt.Println(color.CyanString("[DRY-RUN] Would run initial sync"))
		fmt.Printf("%s\n", color.CyanString("[DRY-RUN] Would mark project %q as initialized", project.Name))
		return nil
	}

	// Run bisync with resync
	fmt.Println()
	fmt.Println("Running initial sync...")

	opts := sync.BisyncOptions{
		Resync:       true,
		ResyncMode:   resyncMode,
		DryRun:       false,
		Verbose:      verbose,
		SyncrDataDir: cfg.SyncrDataDir(),
		Excludes:     cfg.ResolvedExcludes(project.Name),
	}

	ctx := context.Background()
	result, err := sync.RunBisync(ctx, project.LocalPath, syncPath, opts)
	if err != nil {
		st.RecordError(project.Name, err)
		friendly, _ := sync.FriendlyError(err)
		return fmt.Errorf("%s", friendly)
	}

	fmt.Printf("  Duration: %v\n", result.Duration.Round(1e8))

	// Mark as initialized
	st.MarkInitialized(project.Name)
	st.RecordSuccess(project.Name)

	fmt.Println(color.GreenString("Project %q initialized successfully.", project.Name))
	return nil
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

// createStarterConfig creates an initial syncr.json with a sample project.
func createStarterConfig(cfgPath string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not determine working directory: %v\n", err)
		os.Exit(1)
	}

	cfg := config.Config{
		SyncRoot:            cwd,
		SyncIntervalMinutes: 5,
		Projects: []config.Project{
			{
				Name:      "my-project",
				LocalPath: filepath.Join(cwd, "my-project"),
				SyncPath:  "my-project",
				Enabled:   false,
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(cfgPath)
	fmt.Printf("Created %s\n\n", absPath)
	fmt.Println("Next steps:")
	fmt.Println("  1. Update sync_root to your sync folder path (e.g. OneDrive, Dropbox)")
	fmt.Println("  2. Update the sample project or add your own (set enabled: true)")
	fmt.Println("  3. Run: syncr init <project-name>")
}
