package cmd

import (
	"fmt"
	"os"

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
	fmt.Printf("State:  %s/state.json\n", cfg.SyncrDataDir())
	fmt.Println()
	fmt.Printf("cloud_root: %s\n", cfg.CloudRoot)
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
		fmt.Printf("    cloud:  %s\n", p.CloudSubpath)
		fmt.Println()
	}
}
