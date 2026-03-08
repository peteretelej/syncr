package sync

import (
	"os"
	"path/filepath"
	"time"
)

const trashTimestampFormat = "2006-01-02T15-04-05"

// TrashTimestamp returns a UTC timestamp string suitable for use as a trash
// directory name. The format uses dashes instead of colons to be filesystem-safe.
func TrashTimestamp() string {
	return time.Now().UTC().Format(trashTimestampFormat)
}

// CleanTrash removes trash directories older than retentionDays from trashDir.
// Directory names must parse as timestamps in the trash timestamp format.
// Returns the number of directories deleted and the first error encountered
// (processing continues on error).
func CleanTrash(trashDir string, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	entries, err := os.ReadDir(trashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var deleted int
	var firstErr error

	for _, entry := range entries {
		ts, err := time.Parse(trashTimestampFormat, entry.Name())
		if err != nil {
			continue // skip non-timestamp directories
		}

		if time.Since(ts).Hours()/24 > float64(retentionDays) {
			if err := os.RemoveAll(filepath.Join(trashDir, entry.Name())); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			deleted++
		}
	}

	return deleted, firstErr
}

// TrashStats returns the total number of regular files and their combined size
// in bytes within trashDir. Returns (0, 0, nil) if the directory does not exist.
func TrashStats(trashDir string) (fileCount int, totalBytes int64, err error) {
	if _, statErr := os.Stat(trashDir); os.IsNotExist(statErr) {
		return 0, 0, nil
	}

	err = filepath.WalkDir(trashDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		fileCount++
		totalBytes += info.Size()
		return nil
	})

	return fileCount, totalBytes, err
}
