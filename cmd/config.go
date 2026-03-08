package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peteretelej/syncr/internal/config"
)

// ShowConfig displays the current configuration.
func ShowConfig(configPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Config: %s\n", cfg.Path())
	fmt.Printf("Data:   %s\n", cfg.SyncrDataDir())
	fmt.Println()
	fmt.Printf("sync_root: %s\n", cfg.SyncRoot)
	fmt.Printf("sync_interval: %dm\n", cfg.SyncIntervalMinutes)
	fmt.Println()
	fmt.Printf("Projects (%d):\n", len(cfg.Projects))

	for _, p := range cfg.Projects {
		status := "enabled"
		if !p.Enabled {
			status = "disabled"
		}
		fmt.Printf("  %s (%s)\n", p.Name, status)
		fmt.Printf("    local:  %s\n", p.LocalPath)
		fmt.Printf("    sync folder:  %s\n", filepath.Join(cfg.SyncRoot, p.SyncPath))
		if len(p.Exclude) > 0 {
			fmt.Printf("    exclude: %s\n", strings.Join(p.Exclude, ", "))
		}
		fmt.Println()
	}

	// Run full validation and display issues
	result := cfg.ValidateFull()
	if result.HasIssues() {
		fmt.Println("Validation:")
		for _, e := range result.Errors {
			fmt.Printf("  ERROR  %s\n", e)
		}
		for _, w := range result.Warnings {
			fmt.Printf("  WARN   %s\n", w)
		}
	}
	if !result.OK() {
		os.Exit(1)
	}
}
