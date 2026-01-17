package sync

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// CountConflicts returns the number of conflict files in the given directory tree.
func CountConflicts(path string) (int, error) {
	conflicts, err := ListConflicts(path)
	if err != nil {
		return 0, err
	}
	return len(conflicts), nil
}

// ListConflicts returns all conflict file paths in the given directory tree.
// Conflict files are identified by containing ".conflict" in their name,
// following rclone's default conflict naming pattern (e.g., file.conflict1).
func ListConflicts(path string) ([]string, error) {
	var conflicts []string

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip directories we can't access
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check if filename contains ".conflict"
		if strings.Contains(d.Name(), ".conflict") {
			// Return path relative to the search root
			relPath, err := filepath.Rel(path, p)
			if err != nil {
				relPath = p
			}
			conflicts = append(conflicts, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return conflicts, nil
}

// HasConflicts returns true if there are any conflict files in the directory.
func HasConflicts(path string) (bool, error) {
	count, err := CountConflicts(path)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
