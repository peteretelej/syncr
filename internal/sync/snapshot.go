package sync

import (
	"io/fs"
	"path/filepath"
	"time"
)

// DirSnapshot captures a lightweight summary of a directory's contents
// for detecting whether files changed during a sync operation.
type DirSnapshot struct {
	FileCount     int
	TotalSize     int64
	LatestModTime time.Time
}

// TakeSnapshot walks a directory and returns a snapshot of its contents.
// Only regular files are counted; directories are skipped. If the directory
// does not exist or cannot be read, an error is returned.
func TakeSnapshot(path string) (DirSnapshot, error) {
	var snap DirSnapshot

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil // skip files we can't stat
		}

		snap.FileCount++
		snap.TotalSize += info.Size()
		if info.ModTime().After(snap.LatestModTime) {
			snap.LatestModTime = info.ModTime()
		}

		return nil
	})

	return snap, err
}

// Changed returns true if any field differs between the two snapshots.
// This is an approximation; certain edge cases (e.g., a delete paired with
// a same-size add, or in-place content changes without modtime update) will
// not be detected. This is acceptable since hooks are a convenience feature.
func (s DirSnapshot) Changed(other DirSnapshot) bool {
	return s.FileCount != other.FileCount ||
		s.TotalSize != other.TotalSize ||
		!s.LatestModTime.Equal(other.LatestModTime)
}
