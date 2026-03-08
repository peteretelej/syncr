package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/cmd/bisync"
	"github.com/rclone/rclone/fs/filter"
)

func TestInit(t *testing.T) {
	// Init should succeed without error
	err := Init()
	if err != nil {
		t.Errorf("Init failed: %v", err)
	}

	// Second call should also succeed (idempotent)
	err = Init()
	if err != nil {
		t.Errorf("Second Init failed: %v", err)
	}
}

func TestValidatePath(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "syncr-bisync-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Valid directory should pass
	err = validatePath(tmpDir)
	if err != nil {
		t.Errorf("validatePath failed for valid directory: %v", err)
	}

	// Non-existent path should fail
	nonexistentPath := filepath.Join(tmpDir, "nonexistent", "path")
	err = validatePath(nonexistentPath)
	if err == nil {
		t.Error("validatePath should fail for non-existent path")
	}

	// File (not directory) should fail
	tmpFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	err = validatePath(tmpFile)
	if err == nil {
		t.Error("validatePath should fail for file (not directory)")
	}
}

func TestRunBisync_InvalidPaths(t *testing.T) {
	ctx := context.Background()

	// Create only the local path
	tmpDir, err := os.MkdirTemp("", "syncr-bisync-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	localPath := filepath.Join(tmpDir, "local")
	if err := os.MkdirAll(localPath, 0755); err != nil {
		t.Fatal(err)
	}

	opts := BisyncOptions{
		SyncrDataDir: filepath.Join(tmpDir, "_syncr"),
	}

	// Test with non-existent cloud path
	nonexistentCloud := filepath.Join(tmpDir, "nonexistent", "cloud")
	_, err = RunBisync(ctx, localPath, nonexistentCloud, opts)
	if err == nil {
		t.Error("RunBisync should fail with non-existent cloud path")
	}

	// Test with non-existent local path
	nonexistentLocal := filepath.Join(tmpDir, "nonexistent", "local")
	_, err = RunBisync(ctx, nonexistentLocal, localPath, opts)
	if err == nil {
		t.Error("RunBisync should fail with non-existent local path")
	}
}

func TestResyncMode(t *testing.T) {
	// Test that ResyncMode constants have expected values
	if ResyncNone != 0 {
		t.Errorf("ResyncNone should be 0, got %d", ResyncNone)
	}
	if ResyncPath1 != 1 {
		t.Errorf("ResyncPath1 should be 1, got %d", ResyncPath1)
	}
	if ResyncPath2 != 2 {
		t.Errorf("ResyncPath2 should be 2, got %d", ResyncPath2)
	}
}

func TestBisyncOptions_Defaults(t *testing.T) {
	opts := BisyncOptions{}

	// Default values should be zero/false
	if opts.Resync {
		t.Error("Resync should default to false")
	}
	if opts.DryRun {
		t.Error("DryRun should default to false")
	}
	if opts.Verbose {
		t.Error("Verbose should default to false")
	}
	if opts.ResyncMode != ResyncNone {
		t.Error("ResyncMode should default to ResyncNone")
	}
}

func TestFilterExcludes(t *testing.T) {
	ctx := context.Background()
	var fi *filter.Filter
	ctx, fi = filter.AddConfig(ctx)

	// Add exclude patterns
	if err := fi.Add(false, "*.db"); err != nil {
		t.Fatalf("failed to add exclude pattern: %v", err)
	}
	if err := fi.Add(false, ".cache/"); err != nil {
		t.Fatalf("failed to add exclude pattern: %v", err)
	}

	_ = ctx // ctx carries the filter; we test the filter directly

	// Excluded files should not be included
	if fi.IncludeRemote("test.db") {
		t.Error("expected test.db to be excluded")
	}

	// Non-excluded files should be included
	if !fi.IncludeRemote("test.md") {
		t.Error("expected test.md to be included")
	}
	if !fi.IncludeRemote("test.txt") {
		t.Error("expected test.txt to be included")
	}
}

// TestRunBisync_Integration is an integration test that performs actual bisync.
// Run with: go test -v -run TestRunBisync_Integration -tags=integration
func TestRunBisync_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "syncr-bisync-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	localPath := filepath.Join(tmpDir, "local")
	cloudPath := filepath.Join(tmpDir, "cloud")
	syncrDataDir := filepath.Join(tmpDir, "_syncr")

	if err := os.MkdirAll(localPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cloudPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test file in local
	testFile := filepath.Join(localPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := BisyncOptions{
		Resync:       true,
		ResyncMode:   ResyncPath1,
		DryRun:       false,
		SyncrDataDir: syncrDataDir,
	}

	// Run bisync
	result, err := RunBisync(ctx, localPath, cloudPath, opts)
	if err != nil {
		t.Fatalf("RunBisync failed: %v", err)
	}

	if !result.Success {
		t.Errorf("RunBisync should succeed, error: %s", result.Error)
	}

	// Verify file was synced to cloud
	cloudFile := filepath.Join(cloudPath, "test.txt")
	if _, err := os.Stat(cloudFile); os.IsNotExist(err) {
		t.Error("File should have been synced to cloud")
	}
}

// TestRunBisync_Integration_Excludes verifies that excluded files are not synced.
func TestRunBisync_Integration_Excludes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "syncr-bisync-excludes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	localPath := filepath.Join(tmpDir, "local")
	cloudPath := filepath.Join(tmpDir, "cloud")
	syncrDataDir := filepath.Join(tmpDir, "_syncr")

	if err := os.MkdirAll(localPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cloudPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test files in local: one to sync, one to exclude
	if err := os.WriteFile(filepath.Join(localPath, "readme.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localPath, "data.db"), []byte("db content"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := BisyncOptions{
		Resync:       true,
		ResyncMode:   ResyncPath1,
		DryRun:       false,
		SyncrDataDir: syncrDataDir,
		Excludes:     []string{"*.db"},
	}

	// Run bisync with exclude
	result, err := RunBisync(ctx, localPath, cloudPath, opts)
	if err != nil {
		t.Fatalf("RunBisync failed: %v", err)
	}

	if !result.Success {
		t.Errorf("RunBisync should succeed, error: %s", result.Error)
	}

	// Verify the txt file WAS synced
	if _, err := os.Stat(filepath.Join(cloudPath, "readme.txt")); os.IsNotExist(err) {
		t.Error("readme.txt should have been synced to cloud")
	}

	// Verify the db file was NOT synced
	if _, err := os.Stat(filepath.Join(cloudPath, "data.db")); !os.IsNotExist(err) {
		t.Error("data.db should NOT have been synced to cloud (excluded)")
	}
}

func TestBuildBisyncOpts_ConflictResolve(t *testing.T) {
	opts := BisyncOptions{
		ConflictResolve: "newer",
	}
	result, err := buildBisyncOpts(opts, "/tmp/workdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ConflictResolve != bisync.PreferNewer {
		t.Errorf("ConflictResolve = %v, want %v (PreferNewer)", result.ConflictResolve, bisync.PreferNewer)
	}
}

func TestBuildBisyncOpts_ConflictResolveEmpty(t *testing.T) {
	opts := BisyncOptions{}
	result, err := buildBisyncOpts(opts, "/tmp/workdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ConflictResolve != bisync.PreferNone {
		t.Errorf("ConflictResolve = %v, want %v (PreferNone)", result.ConflictResolve, bisync.PreferNone)
	}
}

func TestBuildBisyncOpts_ConflictResolveInvalid(t *testing.T) {
	opts := BisyncOptions{
		ConflictResolve: "bogus",
	}
	_, err := buildBisyncOpts(opts, "/tmp/workdir")
	if err == nil {
		t.Fatal("expected error for invalid conflict_resolve, got nil")
	}
}

func TestBuildBisyncOpts_ConflictSuffix(t *testing.T) {
	opts := BisyncOptions{
		ConflictSuffix: "{DateOnly}",
	}
	result, err := buildBisyncOpts(opts, "/tmp/workdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ConflictSuffixFlag != "{DateOnly}" {
		t.Errorf("ConflictSuffixFlag = %q, want %q", result.ConflictSuffixFlag, "{DateOnly}")
	}
}

func TestBuildBisyncOpts_ConflictSuffixEmpty(t *testing.T) {
	opts := BisyncOptions{}
	result, err := buildBisyncOpts(opts, "/tmp/workdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ConflictSuffixFlag != "" {
		t.Errorf("ConflictSuffixFlag = %q, want %q", result.ConflictSuffixFlag, "")
	}
}

func TestBuildBisyncOpts_ExistingBehavior(t *testing.T) {
	opts := BisyncOptions{
		Resync:     true,
		ResyncMode: ResyncPath1,
		DryRun:     true,
		BackupDir1: "/backup/dir1",
		BackupDir2: "/backup/dir2",
	}
	workdir := "/tmp/test-workdir"
	result, err := buildBisyncOpts(opts, workdir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Workdir != workdir {
		t.Errorf("Workdir = %q, want %q", result.Workdir, workdir)
	}
	if result.MaxDelete != 50 {
		t.Errorf("MaxDelete = %d, want 50", result.MaxDelete)
	}
	if !result.Resilient {
		t.Error("Resilient should be true")
	}
	if !result.Recover {
		t.Error("Recover should be true")
	}
	if !result.DryRun {
		t.Error("DryRun should be true")
	}
	if result.CheckSync != bisync.CheckSyncTrue {
		t.Errorf("CheckSync = %v, want CheckSyncTrue", result.CheckSync)
	}
	if !result.Compare.Size || !result.Compare.Modtime {
		t.Error("Compare should have Size and Modtime enabled")
	}
	if result.CompareFlag != "size,modtime" {
		t.Errorf("CompareFlag = %q, want %q", result.CompareFlag, "size,modtime")
	}
	if !result.Resync {
		t.Error("Resync should be true")
	}
	if result.ResyncMode != bisync.PreferPath1 {
		t.Errorf("ResyncMode = %v, want PreferPath1", result.ResyncMode)
	}
	if result.BackupDir1 != "/backup/dir1" {
		t.Errorf("BackupDir1 = %q, want %q", result.BackupDir1, "/backup/dir1")
	}
	if result.BackupDir2 != "/backup/dir2" {
		t.Errorf("BackupDir2 = %q, want %q", result.BackupDir2, "/backup/dir2")
	}
}
