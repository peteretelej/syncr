package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
	stateDir, err := config.StateDir()
	if err != nil {
		fmt.Printf("State:  (error: %v)\n", err)
	} else {
		fmt.Printf("State:  %s\n", filepath.Join(stateDir, "state.json"))
	}
	fmt.Println()
	fmt.Printf("sync_root: %s\n", cfg.SyncRoot)
	fmt.Printf("sync_interval: %ds\n", cfg.SyncIntervalSeconds)
	fmt.Println()
	fmt.Printf("Projects (%d):\n", len(cfg.Projects))

	for _, p := range cfg.Projects {
		status := "enabled"
		if !p.Enabled {
			status = "disabled"
		}
		fmt.Printf("  %s (%s)\n", p.Name, status)
		fmt.Printf("    local:  %s\n", p.LocalPath)
		fmt.Printf("    cloud:  %s\n", p.SyncPath)
		fmt.Println()
	}
}
