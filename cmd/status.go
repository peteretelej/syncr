package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
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
	st, err := state.Load(cfg.SyncrDataDir())
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

	// Check if any project has excludes
	hasExcludes := false
	for _, p := range cfg.Projects {
		if len(p.Exclude) > 0 {
			hasExcludes = true
			break
		}
	}

	// Header
	if hasExcludes {
		fmt.Printf("  %-*s  %-16s  %-18s  %-9s  %s\n", maxNameLen, "Name", "Status", "Last Sync", "Conflicts", "Excludes")
		fmt.Printf("  %s  %s  %s  %s  %s\n",
			repeatStr("-", maxNameLen),
			repeatStr("-", 16),
			repeatStr("-", 18),
			repeatStr("-", 9),
			repeatStr("-", 8))
	} else {
		fmt.Printf("  %-*s  %-16s  %-18s  %s\n", maxNameLen, "Name", "Status", "Last Sync", "Conflicts")
		fmt.Printf("  %s  %s  %s  %s\n",
			repeatStr("-", maxNameLen),
			repeatStr("-", 16),
			repeatStr("-", 18),
			repeatStr("-", 9))
	}

	// Collect conflicts for later display
	var allConflicts []conflictInfo

	for _, project := range cfg.Projects {
		ps := st.GetProject(project.Name)
		syncPath := filepath.Join(cfg.SyncRoot, project.SyncPath)

		// Determine status
		var statusPlain string
		if !project.Enabled {
			statusPlain = "disabled"
		} else if !ps.Initialized {
			statusPlain = "not initialized"
		} else if ps.LastSyncStatus == "error" {
			statusPlain = fmt.Sprintf("%d errors", ps.ErrorCount)
		} else if ps.LastSyncStatus == "conflicts" {
			statusPlain = "conflicts"
		} else {
			statusPlain = "synced"
		}

		// Pad plain string first, then colorize (ANSI codes break %-*s padding)
		paddedStatus := fmt.Sprintf("%-16s", statusPlain)
		coloredStatus := colorizeStatus(paddedStatus, statusPlain)

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
			conflicts, _ = sync.ListConflicts(syncPath, cfg.ResolvedConflictSuffix())
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
			conflictStr = color.YellowString("%d", conflictCount)
		}

		if hasExcludes {
			excludeStr := "-"
			if len(project.Exclude) > 0 {
				excludeStr = fmt.Sprintf("%d excludes", len(project.Exclude))
			}
			fmt.Printf("  %-*s  %s  %-18s  %-9s  %s\n",
				maxNameLen, project.Name, coloredStatus, lastSync, conflictStr, excludeStr)
		} else {
			fmt.Printf("  %-*s  %s  %-18s  %s\n",
				maxNameLen, project.Name, coloredStatus, lastSync, conflictStr)
		}
	}

	// Show conflict details if any
	if len(allConflicts) > 0 {
		fmt.Println()
		fmt.Println(color.YellowString("Conflict files:"))
		for _, ci := range allConflicts {
			fmt.Printf("  %s:\n", ci.project)
			for _, f := range ci.conflicts {
				fmt.Printf("    %s\n", f)
			}
		}
		fmt.Println()
		suffix := cfg.ResolvedConflictSuffix()
		if suffix == "" {
			suffix = "conflict"
		}
		fmt.Println(color.YellowString("Resolve by keeping the version you want and deleting .%s files.", suffix))
	}

	// Show trash stats for backup-enabled projects
	hasBackup := false
	for _, p := range cfg.Projects {
		if p.BackupDir {
			hasBackup = true
			break
		}
	}
	if hasBackup {
		fmt.Println()
		fmt.Println("Backup trash:")
		for _, project := range cfg.Projects {
			if !project.BackupDir {
				continue
			}
			trashPath := cfg.TrashDir(project.Name)
			fileCount, totalBytes, err := sync.TrashStats(trashPath)
			if err != nil {
				fmt.Printf("  Trash: error reading (%s)\n", trashPath)
			} else if fileCount > 0 {
				fmt.Printf("  %s:\n", project.Name)
				fmt.Printf("    %d files, %s (%s)\n", fileCount, formatSize(totalBytes), trashPath)
			} else {
				fmt.Printf("  Trash: empty (%s)\n", trashPath)
			}
		}
	}

	// Show derived file info if any project has derived entries
	hasDerived := false
	for _, p := range cfg.Projects {
		if len(p.Derived) > 0 {
			hasDerived = true
			break
		}
	}
	if hasDerived {
		fmt.Println()
		fmt.Println("Derived files:")
		for _, p := range cfg.Projects {
			if len(p.Derived) == 0 {
				continue
			}
			fmt.Printf("  %s:\n", p.Name)
			for pattern, description := range p.Derived {
				fmt.Printf("    %s - %s\n", pattern, description)
			}
		}
	}
}

type conflictInfo struct {
	project   string
	conflicts []string
}

// colorizeStatus applies color to a pre-padded status string based on the plain status value.
func colorizeStatus(padded, plain string) string {
	switch {
	case plain == "synced":
		return color.GreenString(padded)
	case plain == "disabled":
		return color.New(color.Faint).Sprint(padded)
	case plain == "not initialized":
		return color.RedString(padded)
	case plain == "conflicts":
		return color.YellowString(padded)
	case len(plain) >= 6 && plain[len(plain)-6:] == "errors":
		return color.RedString(padded)
	default:
		return padded
	}
}

// checkDaemonStatus checks if the daemon is running by looking for a PID file.
func checkDaemonStatus(syncrDataDir string) string {
	pidFile := filepath.Join(syncrDataDir, "syncr.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return color.New(color.Faint).Sprint("not running")
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return color.New(color.Faint).Sprint("not running")
	}

	if !isProcessAlive(pid) {
		return color.New(color.Faint).Sprint("not running")
	}

	return color.GreenString("running (pid %d)", pid)
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

// formatSize formats a byte count as a human-readable string.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes < KB:
		return fmt.Sprintf("%d B", bytes)
	case bytes < MB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	case bytes < GB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	default:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	}
}
