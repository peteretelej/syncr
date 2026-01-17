package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCountConflicts(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "syncr-conflicts-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := []string{
		"normal.txt",
		"document.conflict1",
		"document.conflict2",
		"subdir/file.txt",
		"subdir/notes.conflict1",
	}

	for _, f := range testFiles {
		path := filepath.Join(tmpDir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Test count
	count, err := CountConflicts(tmpDir)
	if err != nil {
		t.Fatalf("CountConflicts failed: %v", err)
	}

	expected := 3 // document.conflict1, document.conflict2, subdir/notes.conflict1
	if count != expected {
		t.Errorf("expected %d conflicts, got %d", expected, count)
	}
}

func TestListConflicts(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "syncr-conflicts-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := []string{
		"normal.txt",
		"document.conflict1",
		"subdir/notes.conflict1",
	}

	for _, f := range testFiles {
		path := filepath.Join(tmpDir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Test list
	conflicts, err := ListConflicts(tmpDir)
	if err != nil {
		t.Fatalf("ListConflicts failed: %v", err)
	}

	expected := 2
	if len(conflicts) != expected {
		t.Errorf("expected %d conflicts, got %d: %v", expected, len(conflicts), conflicts)
	}

	// Check that we get relative paths
	for _, c := range conflicts {
		if filepath.IsAbs(c) {
			t.Errorf("expected relative path, got absolute: %s", c)
		}
	}
}

func TestHasConflicts(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "syncr-conflicts-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with no conflicts
	hasConflicts, err := HasConflicts(tmpDir)
	if err != nil {
		t.Fatalf("HasConflicts failed: %v", err)
	}
	if hasConflicts {
		t.Error("expected no conflicts in empty directory")
	}

	// Add a conflict file
	if err := os.WriteFile(filepath.Join(tmpDir, "file.conflict1"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	hasConflicts, err = HasConflicts(tmpDir)
	if err != nil {
		t.Fatalf("HasConflicts failed: %v", err)
	}
	if !hasConflicts {
		t.Error("expected conflicts after adding conflict file")
	}
}

func TestListConflicts_EmptyDir(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "syncr-conflicts-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	conflicts, err := ListConflicts(tmpDir)
	if err != nil {
		t.Fatalf("ListConflicts failed: %v", err)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts in empty directory, got %d", len(conflicts))
	}
}

func TestListConflicts_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistentPath := filepath.Join(tmpDir, "nonexistent", "path")
	_, err := ListConflicts(nonexistentPath)
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}
