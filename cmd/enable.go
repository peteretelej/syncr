package cmd

import (
	"fmt"
	"os"

	"github.com/peteretelej/syncr/internal/config"
)

// SetProjectEnabled enables or disables a project in the config file.
func SetProjectEnabled(args []string, configPath string, enabled bool) {
	action := "enable"
	if !enabled {
		action = "disable"
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: syncr %s <project>\n", action)
		os.Exit(1)
	}

	name := args[0]

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	project := cfg.GetProject(name)
	if project == nil {
		fmt.Fprintf(os.Stderr, "Error: project %q not found\n", name)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Available projects:")
		for _, p := range cfg.Projects {
			fmt.Fprintf(os.Stderr, "  %s\n", p.Name)
		}
		os.Exit(1)
	}

	if project.Enabled == enabled {
		state := "enabled"
		if !enabled {
			state = "disabled"
		}
		fmt.Printf("Project %q is already %s\n", name, state)
		return
	}

	project.Enabled = enabled
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	past := "Enabled"
	if !enabled {
		past = "Disabled"
	}
	fmt.Printf("%s project %q\n", past, name)
}
