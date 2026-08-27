package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTakeSnapshot_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	snap, err := TakeSnapshot(dir)
	if err != nil {
		t.Fatalf("TakeSnapshot: unexpected error: %v", err)
	}
	if snap.FileCount != 0 {
		t.Errorf("FileCount = %d, want 0", snap.FileCount)
	}
	if snap.TotalSize != 0 {
		t.Errorf("TotalSize = %d, want 0", snap.TotalSize)
	}
	if !snap.LatestModTime.IsZero() {
		t.Errorf("LatestModTime = %v, want zero", snap.LatestModTime)
	}
}

func TestTakeSnapshot_WithFiles(t *testing.T) {
	dir := t.TempDir()

	// Create files with known sizes
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world!"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with a file
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	if err := os.WriteFile(filepath.Join(sub, "c.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	snap, err := TakeSnapshot(dir)
	if err != nil {
		t.Fatalf("TakeSnapshot: unexpected error: %v", err)
	}
	if snap.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", snap.FileCount)
	}
	// "hello" (5) + "world!" (6) + "hi" (2) = 13
	if snap.TotalSize != 13 {
		t.Errorf("TotalSize = %d, want 13", snap.TotalSize)
	}
	if snap.LatestModTime.IsZero() {
		t.Error("LatestModTime should not be zero")
	}
}

func TestTakeSnapshot_Excludes(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"keep.txt":          "keep",
		"ignored.tmp":       "ignored",
		".cache/nested.txt": "ignored",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	snap, err := TakeSnapshot(dir, "*.tmp", ".cache/")
	if err != nil {
		t.Fatalf("TakeSnapshot: unexpected error: %v", err)
	}
	if snap.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", snap.FileCount)
	}
	if snap.TotalSize != int64(len("keep")) {
		t.Errorf("TotalSize = %d, want %d", snap.TotalSize, len("keep"))
	}

	if err := os.WriteFile(filepath.Join(dir, "ignored.tmp"), []byte("changed excluded content"), 0644); err != nil {
		t.Fatal(err)
	}
	after, err := TakeSnapshot(dir, "*.tmp", ".cache/")
	if err != nil {
		t.Fatalf("TakeSnapshot after excluded change: %v", err)
	}
	if snap.Changed(after) {
		t.Error("snapshot changed after only an excluded file changed")
	}
}

func TestDirSnapshot_Changed_Identical(t *testing.T) {
	now := time.Now()
	a := DirSnapshot{FileCount: 5, TotalSize: 100, LatestModTime: now}
	b := DirSnapshot{FileCount: 5, TotalSize: 100, LatestModTime: now}

	if a.Changed(b) {
		t.Error("Changed() = true for identical snapshots, want false")
	}
}

func TestDirSnapshot_Changed_FileCount(t *testing.T) {
	now := time.Now()
	a := DirSnapshot{FileCount: 5, TotalSize: 100, LatestModTime: now}
	b := DirSnapshot{FileCount: 6, TotalSize: 100, LatestModTime: now}

	if !a.Changed(b) {
		t.Error("Changed() = false when FileCount differs, want true")
	}
}

func TestDirSnapshot_Changed_TotalSize(t *testing.T) {
	now := time.Now()
	a := DirSnapshot{FileCount: 5, TotalSize: 100, LatestModTime: now}
	b := DirSnapshot{FileCount: 5, TotalSize: 200, LatestModTime: now}

	if !a.Changed(b) {
		t.Error("Changed() = false when TotalSize differs, want true")
	}
}

func TestDirSnapshot_Changed_ModTime(t *testing.T) {
	now := time.Now()
	a := DirSnapshot{FileCount: 5, TotalSize: 100, LatestModTime: now}
	b := DirSnapshot{FileCount: 5, TotalSize: 100, LatestModTime: now.Add(time.Second)}

	if !a.Changed(b) {
		t.Error("Changed() = false when LatestModTime differs, want true")
	}
}

func TestTakeSnapshot_NonexistentDir(t *testing.T) {
	_, err := TakeSnapshot(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Error("TakeSnapshot: expected error for nonexistent directory, got nil")
	}
}
