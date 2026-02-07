package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/state"
)

// Add adds a new project to the config interactively and initializes it.
func Add(args []string, configPath string, verbose, dryRun bool) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: syncr add <name> [local-path]")
		os.Exit(1)
	}

	name := args[0]

	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Get local path from argument or prompt
	var localPath string
	hasLocalPathArg := len(args) > 1
	if hasLocalPathArg {
		localPath = args[1]
	} else {
		localPath = prompt("Local path: ")
		if localPath == "" {
			fmt.Fprintln(os.Stderr, "Error: local path is required")
			os.Exit(1)
		}
	}

	// Validate and add project to config
	if err := addToConfig(cfg, name, localPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Show confirmation (skip when both name and path are provided as args)
	if !hasLocalPathArg {
		project := cfg.GetProject(name)
		fmt.Printf("\nAdd project %q?\n", name)
		fmt.Printf("  Local:  %s\n", project.LocalPath)
		fmt.Printf("  Cloud:  %s/%s\n", cfg.SyncRoot, project.SyncPath)
		if !confirm("Confirm?") {
			fmt.Println("Cancelled.")
			return
		}
		fmt.Println()
	}

	// Save config
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added project %q to config.\n", name)

	// Initialize the project
	project := cfg.GetProject(name)

	st, err := state.LoadWithMigration(cfg.SyncrDataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	if err := initProject(cfg, st, project, verbose, dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing project: %v\n", err)
		os.Exit(1)
	}

	if !dryRun {
		if err := st.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			os.Exit(1)
		}
	}
}

// addToConfig validates inputs and appends a new project to the config.
// Does not save the config to disk - the caller decides when to save.
func addToConfig(cfg *config.Config, name, localPath string) error {
	// Check name doesn't already exist
	if existing := cfg.GetProject(name); existing != nil {
		return fmt.Errorf("project %q already exists in config", name)
	}

	// Check sync_path (= name) doesn't overlap with existing sync_paths
	syncPath := name
	normalized := filepath.Clean(syncPath)
	for _, p := range cfg.Projects {
		existingSyncPath := p.SyncPath
		if existingSyncPath == "" {
			existingSyncPath = p.Name
		}
		existingNormalized := filepath.Clean(existingSyncPath)
		if normalized == existingNormalized {
			return fmt.Errorf("sync path %q conflicts with existing project %q", syncPath, p.Name)
		}
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("resolving path: %v", err)
	}

	// Warn if local path doesn't exist (but allow it)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Printf("Warning: local path does not exist: %s\n", absPath)
	}

	// Append project to config
	cfg.Projects = append(cfg.Projects, config.Project{
		Name:      name,
		LocalPath: absPath,
		SyncPath:  syncPath,
		Enabled:   true,
	})

	return nil
}
