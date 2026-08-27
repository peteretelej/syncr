package sync

import (
	"context"
	"fmt"
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

// includedFile reports whether a file passes the exclude filter, matching the
// file rules rclone applies during bisync.
func includedFile(fi *filter.Filter, root, path string) bool {
	if fi == nil || path == root {
		return true
	}

	remote, ok := remotePath(root, path)
	if !ok {
		return false
	}
	return fi.IncludeRemote(remote)
}

// includedDir reports whether a directory should be descended into. It uses
// rclone's directory rules rather than the file rules, so a pattern such as
// "dir/*" (which excludes dir/file.txt but not dir/sub/file.txt) does not
// prune the whole subtree.
func includedDir(fi *filter.Filter, root, path string) bool {
	if fi == nil || path == root {
		return true
	}

	remote, ok := remotePath(root, path)
	if !ok {
		return false
	}
	// The only error source is ExcludeFile lookups, which are never configured
	// here, so the nil Fs is unused and err is always nil in practice.
	include, err := fi.IncludeDirectory(context.Background(), nil)(remote)
	if err != nil {
		return true
	}
	return include
}

func remotePath(root, path string) (string, bool) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(relative), true
}
