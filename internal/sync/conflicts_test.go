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
	count, err := CountConflicts(tmpDir, "")
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
	conflicts, err := ListConflicts(tmpDir, "")
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

func TestListConflicts_Excludes(t *testing.T) {
	tmpDir := t.TempDir()
	testFiles := []string{
		"keep.conflict1",
		"ignored.tmp.conflict1",
		".cache/nested.conflict1",
	}
	for _, name := range testFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	conflicts, err := ListConflicts(tmpDir, "", "*.tmp.conflict*", ".cache/")
	if err != nil {
		t.Fatalf("ListConflicts failed: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0] != "keep.conflict1" {
		t.Errorf("conflicts = %v, want [keep.conflict1]", conflicts)
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
	hasConflicts, err := HasConflicts(tmpDir, "")
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

	hasConflicts, err = HasConflicts(tmpDir, "")
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

	conflicts, err := ListConflicts(tmpDir, "")
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
	_, err := ListConflicts(nonexistentPath, "")
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

// --- New tests for ConflictSuffixPattern ---

func TestConflictSuffixPattern_Default(t *testing.T) {
	re := ConflictSuffixPattern("")

	matches := []string{
		"file.conflict",
		"file.conflict1",
		"file.conflict2",
		"document.txt.conflict1",
	}
	for _, m := range matches {
		if !re.MatchString(m) {
			t.Errorf("expected pattern to match %q", m)
		}
	}

	nonMatches := []string{
		"file.backup1",
		"file.txt",
		"conflict1",
	}
	for _, nm := range nonMatches {
		if re.MatchString(nm) {
			t.Errorf("expected pattern NOT to match %q", nm)
		}
	}
}

func TestConflictSuffixPattern_Static(t *testing.T) {
	re := ConflictSuffixPattern("backup")

	matches := []string{
		"file.backup",
		"file.backup1",
		"file.backup2",
		"document.txt.backup1",
	}
	for _, m := range matches {
		if !re.MatchString(m) {
			t.Errorf("expected pattern to match %q", m)
		}
	}

	nonMatches := []string{
		"file.conflict1",
		"file.txt",
		"backup1",
	}
	for _, nm := range nonMatches {
		if re.MatchString(nm) {
			t.Errorf("expected pattern NOT to match %q", nm)
		}
	}
}

func TestConflictSuffixPattern_DateOnly(t *testing.T) {
	re := ConflictSuffixPattern("{DateOnly}")

	matches := []string{
		"file.2026-03-08",
		"file.2026-03-081",
		"document.txt.2024-12-25",
	}
	for _, m := range matches {
		if !re.MatchString(m) {
			t.Errorf("expected pattern to match %q", m)
		}
	}

	nonMatches := []string{
		"file.conflict1",
		"file.txt",
		"file.2026",
	}
	for _, nm := range nonMatches {
		if re.MatchString(nm) {
			t.Errorf("expected pattern NOT to match %q", nm)
		}
	}
}

func TestConflictSuffixPattern_TimeOnly(t *testing.T) {
	re := ConflictSuffixPattern("{TimeOnly}")

	matches := []string{
		"file.15-30-00",
		"file.00-00-00",
		"document.txt.23-59-59",
	}
	for _, m := range matches {
		if !re.MatchString(m) {
			t.Errorf("expected pattern to match %q", m)
		}
	}

	nonMatches := []string{
		"file.conflict1",
		"file.2026-03-08",
	}
	for _, nm := range nonMatches {
		if re.MatchString(nm) {
			t.Errorf("expected pattern NOT to match %q", nm)
		}
	}
}

func TestConflictSuffixPattern_DateTimeISO(t *testing.T) {
	re := ConflictSuffixPattern("{DateTimeISO}")

	matches := []string{
		"file.2026-03-08T15-30-00",
		"document.txt.2024-12-25T00-00-00",
	}
	for _, m := range matches {
		if !re.MatchString(m) {
			t.Errorf("expected pattern to match %q", m)
		}
	}

	nonMatches := []string{
		"file.conflict1",
		"file.2026-03-08",
		"file.15-30-00",
	}
	for _, nm := range nonMatches {
		if re.MatchString(nm) {
			t.Errorf("expected pattern NOT to match %q", nm)
		}
	}
}

func TestConflictSuffixPattern_UnknownGlob(t *testing.T) {
	re := ConflictSuffixPattern("{Custom}")

	matches := []string{
		"file.anything",
		"file.foobar",
		"document.txt.xyz",
	}
	for _, m := range matches {
		if !re.MatchString(m) {
			t.Errorf("expected pattern to match %q", m)
		}
	}

	nonMatches := []string{
		// Must have a dot before the segment
		"file",
	}
	for _, nm := range nonMatches {
		if re.MatchString(nm) {
			t.Errorf("expected pattern NOT to match %q", nm)
		}
	}
}

func TestListConflicts_DefaultSuffix(t *testing.T) {
	tmpDir := t.TempDir()

	// Create conflict and non-conflict files
	files := map[string]bool{
		"normal.txt":           false,
		"file.conflict1":       true,
		"subdir/doc.conflict2": true,
	}

	for name := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	conflicts, err := ListConflicts(tmpDir, "")
	if err != nil {
		t.Fatalf("ListConflicts failed: %v", err)
	}

	if len(conflicts) != 2 {
		t.Errorf("expected 2 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestListConflicts_CustomStaticSuffix(t *testing.T) {
	tmpDir := t.TempDir()

	files := []string{
		"normal.txt",
		"file.backup1",
		"subdir/doc.backup2",
		"file.conflict1", // should NOT match when suffix is "backup"
	}

	for _, name := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	conflicts, err := ListConflicts(tmpDir, "backup")
	if err != nil {
		t.Fatalf("ListConflicts failed: %v", err)
	}

	if len(conflicts) != 2 {
		t.Errorf("expected 2 backup conflicts, got %d: %v", len(conflicts), conflicts)
	}

	// Verify .conflict1 is NOT in the results
	for _, c := range conflicts {
		if c == "file.conflict1" {
			t.Error("file.conflict1 should not match when suffix is 'backup'")
		}
	}
}

func TestListConflicts_DateOnlySuffix(t *testing.T) {
	tmpDir := t.TempDir()

	files := []string{
		"normal.txt",
		"file.2026-03-08",
		"subdir/doc.2026-03-081",
		"file.conflict1", // should NOT match
	}

	for _, name := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	conflicts, err := ListConflicts(tmpDir, "{DateOnly}")
	if err != nil {
		t.Fatalf("ListConflicts failed: %v", err)
	}

	if len(conflicts) != 2 {
		t.Errorf("expected 2 date conflicts, got %d: %v", len(conflicts), conflicts)
	}

	// Verify .conflict1 is NOT in the results
	for _, c := range conflicts {
		if c == "file.conflict1" {
			t.Error("file.conflict1 should not match when suffix is '{DateOnly}'")
		}
	}
}

func TestListConflicts_NoFalsePositives(t *testing.T) {
	tmpDir := t.TempDir()

	// Files that should never match any suffix pattern
	nonConflictFiles := []string{
		"normal.txt",
		"readme.md",
		"subdir/data.json",
		"image.png",
	}

	for _, name := range nonConflictFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Test with default suffix
	conflicts, err := ListConflicts(tmpDir, "")
	if err != nil {
		t.Fatalf("ListConflicts (default) failed: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts with default suffix, got %d: %v", len(conflicts), conflicts)
	}

	// Test with custom static suffix
	conflicts, err = ListConflicts(tmpDir, "backup")
	if err != nil {
		t.Fatalf("ListConflicts (backup) failed: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts with backup suffix, got %d: %v", len(conflicts), conflicts)
	}

	// Test with DateOnly suffix
	conflicts, err = ListConflicts(tmpDir, "{DateOnly}")
	if err != nil {
		t.Fatalf("ListConflicts (DateOnly) failed: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts with DateOnly suffix, got %d: %v", len(conflicts), conflicts)
	}
}
