package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
	err = validatePath("/nonexistent/path/12345")
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
	_, err = RunBisync(ctx, localPath, "/nonexistent/cloud/path", opts)
	if err == nil {
		t.Error("RunBisync should fail with non-existent cloud path")
	}

	// Test with non-existent local path
	_, err = RunBisync(ctx, "/nonexistent/local/path", localPath, opts)
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
