package sync

import "strings"

// errorMapping maps rclone error substrings to user-friendly messages.
var errorMapping = []struct {
	pattern  string
	friendly string
}{
	{"bisync aborted", "Sync aborted due to too many changes (safety limit)"},
	{"directory not found", "Sync folder not accessible"},
	{"permission denied", "Permission denied accessing files"},
}

// FriendlyError translates a raw rclone error into a user-friendly message.
// Returns (friendly message, raw error string). The raw string is suitable for
// log files or verbose output.
func FriendlyError(err error) (friendly, raw string) {
	raw = err.Error()
	lower := strings.ToLower(raw)

	for _, m := range errorMapping {
		if strings.Contains(lower, m.pattern) {
			return m.friendly, raw
		}
	}

	return "Sync failed", raw
}
