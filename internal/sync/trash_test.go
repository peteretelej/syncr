package sync

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestTrashTimestamp(t *testing.T) {
	ts := TrashTimestamp()

	// Must match expected pattern: YYYY-MM-DDTHH-MM-SS
	pattern := `^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}$`
	matched, err := regexp.MatchString(pattern, ts)
	if err != nil {
		t.Fatalf("regex error: %v", err)
	}
	if !matched {
		t.Errorf("timestamp %q does not match pattern %s", ts, pattern)
	}

	// Must not contain colons or spaces
	for _, ch := range ts {
		if ch == ':' || ch == ' ' {
			t.Errorf("timestamp %q contains invalid character %q", ts, string(ch))
		}
	}

	// Must parse back successfully
	parsed, err := time.Parse(trashTimestampFormat, ts)
	if err != nil {
		t.Fatalf("timestamp %q does not parse back: %v", ts, err)
	}

	// Should be close to now (within 2 seconds)
	if time.Since(parsed) > 2*time.Second {
		t.Errorf("parsed timestamp %v is too far from now", parsed)
	}
}

func TestCleanTrash_DeletesOld(t *testing.T) {
	trashDir := t.TempDir()

	// Create a directory 40 days old
	oldTime := time.Now().UTC().Add(-40 * 24 * time.Hour)
	oldName := oldTime.Format(trashTimestampFormat)
	oldPath := filepath.Join(trashDir, oldName)
	if err := os.MkdirAll(oldPath, 0755); err != nil {
		t.Fatal(err)
	}
	// Put a file inside to verify RemoveAll works
	if err := os.WriteFile(filepath.Join(oldPath, "file.txt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	deleted, err := CleanTrash(trashDir, 31)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Directory should be gone
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old directory should have been removed")
	}
}

func TestCleanTrash_KeepsRecent(t *testing.T) {
	trashDir := t.TempDir()

	// Create a directory 5 days old (well within 30-day retention)
	recentTime := time.Now().UTC().Add(-5 * 24 * time.Hour)
	recentName := recentTime.Format(trashTimestampFormat)
	recentPath := filepath.Join(trashDir, recentName)
	if err := os.MkdirAll(recentPath, 0755); err != nil {
		t.Fatal(err)
	}

	deleted, err := CleanTrash(trashDir, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}

	// Directory should still exist
	if _, err := os.Stat(recentPath); err != nil {
		t.Error("recent directory should still exist")
	}
}

func TestCleanTrash_ZeroRetention(t *testing.T) {
	trashDir := t.TempDir()

	// Create a very old directory
	oldTime := time.Now().UTC().Add(-365 * 24 * time.Hour)
	oldName := oldTime.Format(trashTimestampFormat)
	if err := os.MkdirAll(filepath.Join(trashDir, oldName), 0755); err != nil {
		t.Fatal(err)
	}

	deleted, err := CleanTrash(trashDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted with zero retention, got %d", deleted)
	}
}

func TestCleanTrash_NegativeRetention(t *testing.T) {
	trashDir := t.TempDir()

	// Create a very old directory
	oldTime := time.Now().UTC().Add(-365 * 24 * time.Hour)
	oldName := oldTime.Format(trashTimestampFormat)
	if err := os.MkdirAll(filepath.Join(trashDir, oldName), 0755); err != nil {
		t.Fatal(err)
	}

	deleted, err := CleanTrash(trashDir, -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted with negative retention, got %d", deleted)
	}
}

func TestCleanTrash_NoDir(t *testing.T) {
	deleted, err := CleanTrash("/nonexistent/path/trash", 30)
	if err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}
}

func TestCleanTrash_InvalidDirNames(t *testing.T) {
	trashDir := t.TempDir()

	// Create directories with non-timestamp names
	for _, name := range []string{"not-a-timestamp", "readme.txt", "2024-13-01T00-00-00", ".DS_Store"} {
		if err := os.MkdirAll(filepath.Join(trashDir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := CleanTrash(trashDir, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted (invalid names should be skipped), got %d", deleted)
	}
}

func TestTrashStats_CountsFilesAndSize(t *testing.T) {
	trashDir := t.TempDir()

	// Create subdirectories with files of known sizes
	sub1 := filepath.Join(trashDir, "2024-01-01T00-00-00")
	sub2 := filepath.Join(trashDir, "2024-01-02T00-00-00")
	if err := os.MkdirAll(sub1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub2, 0755); err != nil {
		t.Fatal(err)
	}

	// Write files: 10 bytes + 20 bytes + 30 bytes = 60 bytes total
	if err := os.WriteFile(filepath.Join(sub1, "a.txt"), make([]byte, 10), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub1, "b.txt"), make([]byte, 20), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub2, "c.txt"), make([]byte, 30), 0644); err != nil {
		t.Fatal(err)
	}

	count, size, err := TrashStats(trashDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 files, got %d", count)
	}
	if size != 60 {
		t.Errorf("expected 60 bytes, got %d", size)
	}
}

func TestTrashStats_NoDir(t *testing.T) {
	count, size, err := TrashStats("/nonexistent/path/trash")
	if err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 files, got %d", count)
	}
	if size != 0 {
		t.Errorf("expected 0 bytes, got %d", size)
	}
}

func TestTrashStats_EmptyDir(t *testing.T) {
	trashDir := t.TempDir()

	count, size, err := TrashStats(trashDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 files, got %d", count)
	}
	if size != 0 {
		t.Errorf("expected 0 bytes, got %d", size)
	}
}
