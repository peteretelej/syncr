package sync

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/rclone/rclone/fs/filter"
)

func newExcludeFilter(excludes []string) (*filter.Filter, error) {
	if len(excludes) == 0 {
		return nil, nil
	}

	fi, err := filter.NewFilter(nil)
	if err != nil {
		return nil, err
	}
	for _, pattern := range excludes {
		if err := fi.Add(false, pattern); err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
		}
	}
	return fi, nil
}

func includedPath(fi *filter.Filter, root, path string, entry fs.DirEntry) bool {
	if fi == nil || path == root {
		return true
	}

	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	remote := filepath.ToSlash(relative)
	if entry.IsDir() {
		remote += "/"
	}
	return fi.IncludeRemote(remote)
}
