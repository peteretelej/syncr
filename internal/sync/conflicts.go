package sync

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

// ConflictSuffixPattern returns a compiled regex that matches filenames
// containing the given conflict suffix. If suffix is empty, matches the
// default rclone pattern (.conflict followed by optional digits).
func ConflictSuffixPattern(suffix string) *regexp.Regexp {
	if suffix == "" {
		suffix = "conflict"
	}

	var pattern string
	if strings.Contains(suffix, "{") {
		// Time-glob suffix: replace known globs with regex equivalents
		escaped := regexp.QuoteMeta(suffix)
		// Restore glob replacements (QuoteMeta escaped the braces)
		escaped = strings.ReplaceAll(escaped, regexp.QuoteMeta("{DateOnly}"), `\d{4}-\d{2}-\d{2}`)
		escaped = strings.ReplaceAll(escaped, regexp.QuoteMeta("{TimeOnly}"), `\d{2}-\d{2}-\d{2}`)
		escaped = strings.ReplaceAll(escaped, regexp.QuoteMeta("{DateTimeISO}"), `\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}`)
		// Unknown {Foo} sequences: match any non-dot segment
		escaped = regexp.MustCompile(`\\\{[^}]*\\\}`).ReplaceAllString(escaped, `[^.]+`)
		pattern = `\.` + escaped + `\d*$`
	} else {
		// Static suffix: match literally with optional trailing digits
		pattern = `\.` + regexp.QuoteMeta(suffix) + `\d*$`
	}

	return regexp.MustCompile(pattern)
}

// CountConflicts returns the number of conflict files in the given directory tree.
// The suffix parameter specifies the conflict suffix to match; use "" for the default
// rclone "conflict" pattern. Optional excludes use rclone filter syntax.
func CountConflicts(path string, suffix string, excludes ...string) (int, error) {
	conflicts, err := ListConflicts(path, suffix, excludes...)
	if err != nil {
		return 0, err
	}
	return len(conflicts), nil
}

// ListConflicts returns all conflict file paths in the given directory tree.
// Conflict files are identified by matching the configured suffix pattern.
// If suffix is empty, the default rclone pattern is used (e.g., file.conflict1).
// Optional excludes use rclone filter syntax.
func ListConflicts(path string, suffix string, excludes ...string) ([]string, error) {
	var conflicts []string
	re := ConflictSuffixPattern(suffix)
	fi, err := newExcludeFilter(excludes)
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip directories we can't access
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return err
		}

		if !includedPath(fi, path, p, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check if filename matches the conflict suffix pattern
		if re.MatchString(d.Name()) {
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
// The suffix parameter specifies the conflict suffix to match; use "" for the default.
// Optional excludes use rclone filter syntax.
func HasConflicts(path string, suffix string, excludes ...string) (bool, error) {
	count, err := CountConflicts(path, suffix, excludes...)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
