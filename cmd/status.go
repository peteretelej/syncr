package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/state"
	"github.com/peteretelej/syncr/internal/sync"
)

// Status shows the current status of all projects.
func Status(configPath string) {
	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Load state
	st, err := state.LoadWithMigration(cfg.SyncrDataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	// Header
	hostname, _ := os.Hostname()
	fmt.Println("syncr status")
	fmt.Printf("Machine: %s\n", hostname)
	fmt.Printf("Config:  %s\n", cfg.Path())
	fmt.Printf("State:   %s\n", st.Path())

	// Check daemon status
	daemonStatus := checkDaemonStatus(cfg.SyncrDataDir())
	fmt.Printf("Daemon:  %s\n", daemonStatus)
	fmt.Println()

	if len(cfg.Projects) == 0 {
		fmt.Println("No projects configured.")
		return
	}

	// Project table
	fmt.Println("Projects:")
	fmt.Println()

	// Calculate column widths
	maxNameLen := 4 // "Name"
	for _, p := range cfg.Projects {
		if len(p.Name) > maxNameLen {
			maxNameLen = len(p.Name)
		}
	}

	// Header
	fmt.Printf("  %-*s  %-12s  %-18s  %s\n", maxNameLen, "Name", "Status", "Last Sync", "Conflicts")
	fmt.Printf("  %s  %s  %s  %s\n",
		repeatStr("-", maxNameLen),
		repeatStr("-", 12),
		repeatStr("-", 18),
		repeatStr("-", 9))

	// Collect conflicts for later display
	var allConflicts []conflictInfo

	for _, project := range cfg.Projects {
		ps := st.GetProject(project.Name)
		syncPath := filepath.Join(cfg.SyncRoot, project.SyncPath)

		// Determine status
		var status string
		if !project.Enabled {
			status = "disabled"
		} else if !ps.Initialized {
			status = "not init"
		} else if ps.LastSyncStatus == "error" {
			status = fmt.Sprintf("error (%d)", ps.ErrorCount)
		} else if ps.LastSyncStatus == "conflicts" {
			status = "conflicts"
		} else {
			status = "synced"
		}

		// Format last sync time
		var lastSync string
		if ps.LastSync.IsZero() {
			lastSync = "-"
		} else {
			lastSync = formatRelativeTime(ps.LastSync)
		}

		// Count conflicts
		conflictCount := 0
		var conflicts []string
		if ps.Initialized && pathExists(syncPath) {
			conflicts, _ = sync.ListConflicts(syncPath)
			conflictCount = len(conflicts)
			if conflictCount > 0 {
				allConflicts = append(allConflicts, conflictInfo{
					project:   project.Name,
					conflicts: conflicts,
				})
			}
		}

		conflictStr := "-"
		if conflictCount > 0 {
			conflictStr = fmt.Sprintf("%d", conflictCount)
		}

		fmt.Printf("  %-*s  %-12s  %-18s  %s\n",
			maxNameLen, project.Name, status, lastSync, conflictStr)
	}

	// Show conflict details if any
	if len(allConflicts) > 0 {
		fmt.Println()
		fmt.Println("Conflict files:")
		for _, ci := range allConflicts {
			fmt.Printf("  %s:\n", ci.project)
			for _, f := range ci.conflicts {
				fmt.Printf("    %s\n", f)
			}
		}
	}
}

type conflictInfo struct {
	project   string
	conflicts []string
}

// checkDaemonStatus checks if the daemon is running by looking for a PID file.
func checkDaemonStatus(syncrDataDir string) string {
	pidFile := filepath.Join(syncrDataDir, "syncr.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return "not running"
	}

	// Check if the process is still running
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return "not running"
	}

	// On Unix, we can check if process exists by sending signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		return "not running"
	}

	// Send signal 0 to check if process exists (Unix only)
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return "not running"
	}

	return fmt.Sprintf("running (pid %d)", pid)
}

// formatRelativeTime formats a time as a relative duration.
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// repeatStr repeats a string n times.
func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
