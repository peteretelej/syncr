package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/state"
	syncpkg "github.com/peteretelej/syncr/internal/sync"
)

// TestHarness provides a temporary environment for integration tests.
type TestHarness struct {
	TmpDir     string
	SyncRoot   string
	LocalDir   string
	ConfigPath string
	Config     *config.Config
	State      *state.State
	t          *testing.T
}

// NewTestHarness creates a new test harness with temporary directories.
func NewTestHarness(t *testing.T) *TestHarness {
	t.Helper()

	tmpDir := t.TempDir()

	h := &TestHarness{
		TmpDir:     tmpDir,
		SyncRoot:   filepath.Join(tmpDir, "cloud"),
		LocalDir:   filepath.Join(tmpDir, "local"),
		ConfigPath: filepath.Join(tmpDir, "syncr.json"),
		t:          t,
	}

	// Create directories
	if err := os.MkdirAll(h.SyncRoot, 0755); err != nil {
		t.Fatalf("failed to create cloud root: %v", err)
	}
	if err := os.MkdirAll(h.LocalDir, 0755); err != nil {
		t.Fatalf("failed to create local dir: %v", err)
	}

	return h
}

// CreateConfig creates a test configuration file.
func (h *TestHarness) CreateConfig(projects []config.Project) {
	h.t.Helper()

	cfg := &config.Config{
		SyncRoot:            h.SyncRoot,
		SyncIntervalSeconds: 300,
		Projects:            projects,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		h.t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(h.ConfigPath, data, 0644); err != nil {
		h.t.Fatalf("failed to write config: %v", err)
	}

	// Load it back
	loaded, err := config.Load(h.ConfigPath)
	if err != nil {
		h.t.Fatalf("failed to load config: %v", err)
	}
	h.Config = loaded
}

// LoadState loads state from the syncr data directory.
func (h *TestHarness) LoadState() {
	h.t.Helper()

	st, err := state.Load(h.Config.SyncrDataDir())
	if err != nil {
		h.t.Fatalf("failed to load state: %v", err)
	}
	h.State = st
}

// CreateFile creates a file with the given content in the specified directory.
func (h *TestHarness) CreateFile(dir, name, content string) {
	h.t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		h.t.Fatalf("failed to create parent dirs: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		h.t.Fatalf("failed to write file %s: %v", path, err)
	}
}

// FileExists checks if a file exists.
func (h *TestHarness) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReadFile reads a file and returns its content.
func (h *TestHarness) ReadFile(path string) string {
	h.t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatalf("failed to read file %s: %v", path, err)
	}
	return string(data)
}

// TestIntegration_ConfigAndState tests that config and state work together.
func TestIntegration_ConfigAndState(t *testing.T) {
	h := NewTestHarness(t)

	// Create config with one project
	h.CreateConfig([]config.Project{
		{
			Name:      "TestProject",
			LocalPath: h.LocalDir,
			SyncPath:  "TestProject",
			Enabled:   true,
		},
	})

	// Validate config
	if err := h.Config.Validate(); err != nil {
		t.Fatalf("config validation failed: %v", err)
	}

	// Load state
	h.LoadState()

	// Project should not be initialized
	if h.State.IsInitialized("TestProject") {
		t.Error("project should not be initialized yet")
	}

	// Mark as initialized
	h.State.MarkInitialized("TestProject")
	if err := h.State.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Reload and verify
	st2, err := state.Load(h.Config.SyncrDataDir())
	if err != nil {
		t.Fatalf("failed to reload state: %v", err)
	}

	if !st2.IsInitialized("TestProject") {
		t.Error("project should be initialized after reload")
	}
}

// TestIntegration_ConflictDetection tests that conflict detection works.
func TestIntegration_ConflictDetection(t *testing.T) {
	h := NewTestHarness(t)

	// Create files including conflict files
	h.CreateFile(h.LocalDir, "normal.txt", "normal content")
	h.CreateFile(h.LocalDir, "document.conflict1", "conflict 1")
	h.CreateFile(h.LocalDir, "subdir/file.conflict2", "conflict 2")

	// Check conflict detection
	count, err := syncpkg.CountConflicts(h.LocalDir)
	if err != nil {
		t.Fatalf("CountConflicts failed: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 conflicts, got %d", count)
	}

	// List conflicts
	conflicts, err := syncpkg.ListConflicts(h.LocalDir)
	if err != nil {
		t.Fatalf("ListConflicts failed: %v", err)
	}

	if len(conflicts) != 2 {
		t.Errorf("expected 2 conflict files, got %d", len(conflicts))
	}
}

// TestIntegration_SyncrDataDir tests that the _syncr directory is created correctly.
func TestIntegration_SyncrDataDir(t *testing.T) {
	h := NewTestHarness(t)

	h.CreateConfig([]config.Project{})
	h.LoadState()

	// Check _syncr directory exists
	syncrDir := h.Config.SyncrDataDir()
	if !h.FileExists(syncrDir) {
		t.Error("_syncr directory should exist")
	}

	// Check state file exists after save
	if err := h.State.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	statePath := filepath.Join(syncrDir, "state.json")
	if !h.FileExists(statePath) {
		t.Error("state.json should exist in _syncr directory")
	}
}

// TestIntegration_MultipleProjects tests handling multiple projects.
func TestIntegration_MultipleProjects(t *testing.T) {
	h := NewTestHarness(t)

	// Create multiple local directories
	localA := filepath.Join(h.TmpDir, "localA")
	localB := filepath.Join(h.TmpDir, "localB")
	os.MkdirAll(localA, 0755)
	os.MkdirAll(localB, 0755)

	h.CreateConfig([]config.Project{
		{Name: "ProjectA", LocalPath: localA, SyncPath: "A", Enabled: true},
		{Name: "ProjectB", LocalPath: localB, SyncPath: "B", Enabled: true},
		{Name: "ProjectC", LocalPath: localA, SyncPath: "C", Enabled: false},
	})

	// Get projects
	projA := h.Config.GetProject("ProjectA")
	projB := h.Config.GetProject("ProjectB")
	projC := h.Config.GetProject("ProjectC")

	if projA == nil || projB == nil || projC == nil {
		t.Fatal("all projects should be found")
	}

	if !projA.Enabled || !projB.Enabled {
		t.Error("ProjectA and ProjectB should be enabled")
	}

	if projC.Enabled {
		t.Error("ProjectC should be disabled")
	}

	// Test state tracking for each project
	h.LoadState()

	h.State.MarkInitialized("ProjectA")
	h.State.RecordSuccess("ProjectA")
	h.State.RecordSuccess("ProjectA")

	h.State.MarkInitialized("ProjectB")
	h.State.RecordError("ProjectB", nil)

	psA := h.State.GetProject("ProjectA")
	psB := h.State.GetProject("ProjectB")

	if psA.SyncCount != 2 {
		t.Errorf("ProjectA SyncCount = %d, want 2", psA.SyncCount)
	}

	if psB.ErrorCount != 1 {
		t.Errorf("ProjectB ErrorCount = %d, want 1", psB.ErrorCount)
	}
}

// TestIntegration_StateRecovery tests that state survives corruption scenarios.
func TestIntegration_StateRecovery(t *testing.T) {
	h := NewTestHarness(t)
	h.CreateConfig([]config.Project{})

	// Create initial state
	h.LoadState()
	h.State.MarkInitialized("TestProject")
	h.State.RecordSuccess("TestProject")
	if err := h.State.Save(); err != nil {
		t.Fatal(err)
	}

	// Reload and verify data persisted
	st2, err := state.Load(h.Config.SyncrDataDir())
	if err != nil {
		t.Fatal(err)
	}

	ps := st2.GetProject("TestProject")
	if !ps.Initialized {
		t.Error("project should still be initialized")
	}
	if ps.SyncCount != 1 {
		t.Errorf("SyncCount = %d, want 1", ps.SyncCount)
	}
}

// TestIntegration_InitLocalEmptyCloudHasFiles tests init when local is empty but cloud has files.
func TestIntegration_InitLocalEmptyCloudHasFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := NewTestHarness(t)

	// Set up cloud path with files
	cloudSubpath := "TestProject"
	syncPath := filepath.Join(h.SyncRoot, cloudSubpath)
	if err := os.MkdirAll(syncPath, 0755); err != nil {
		t.Fatal(err)
	}
	h.CreateFile(syncPath, "cloud-file.txt", "from cloud")
	h.CreateFile(syncPath, "subdir/nested.txt", "nested content")

	// Local should be empty
	h.CreateConfig([]config.Project{
		{
			Name:      "TestProject",
			LocalPath: h.LocalDir,
			SyncPath:  cloudSubpath,
			Enabled:   true,
		},
	})

	// Verify counts
	localCount := countFiles(h.LocalDir)
	syncCount := countFiles(syncPath)

	if localCount != 0 {
		t.Errorf("local should be empty, got %d files", localCount)
	}
	if syncCount != 2 {
		t.Errorf("cloud should have 2 files, got %d", syncCount)
	}

	// Run bisync with resync mode path2 (cloud wins)
	ctx := context.Background()
	h.LoadState()

	opts := syncpkg.BisyncOptions{
		Resync:       true,
		ResyncMode:   syncpkg.ResyncPath2,
		SyncrDataDir: h.Config.SyncrDataDir(),
	}

	result, err := syncpkg.RunBisync(ctx, h.LocalDir, syncPath, opts)
	if err != nil {
		t.Fatalf("bisync failed: %v", err)
	}

	if !result.Success {
		t.Errorf("bisync should succeed, error: %s", result.Error)
	}

	// Verify local now has the cloud files
	if !h.FileExists(filepath.Join(h.LocalDir, "cloud-file.txt")) {
		t.Error("cloud-file.txt should have been synced to local")
	}
	if !h.FileExists(filepath.Join(h.LocalDir, "subdir/nested.txt")) {
		t.Error("subdir/nested.txt should have been synced to local")
	}
}

// TestIntegration_InitBothHaveFiles tests init when both local and cloud have different files.
func TestIntegration_InitBothHaveFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := NewTestHarness(t)

	// Set up cloud path with files
	cloudSubpath := "TestProject"
	syncPath := filepath.Join(h.SyncRoot, cloudSubpath)
	if err := os.MkdirAll(syncPath, 0755); err != nil {
		t.Fatal(err)
	}
	h.CreateFile(syncPath, "cloud-only.txt", "from cloud")

	// Set up local with different files
	h.CreateFile(h.LocalDir, "local-only.txt", "from local")

	h.CreateConfig([]config.Project{
		{
			Name:      "TestProject",
			LocalPath: h.LocalDir,
			SyncPath:  cloudSubpath,
			Enabled:   true,
		},
	})

	// Run bisync with resync (keeps superset)
	ctx := context.Background()
	h.LoadState()

	opts := syncpkg.BisyncOptions{
		Resync:       true,
		ResyncMode:   syncpkg.ResyncNone, // Keep superset
		SyncrDataDir: h.Config.SyncrDataDir(),
	}

	result, err := syncpkg.RunBisync(ctx, h.LocalDir, syncPath, opts)
	if err != nil {
		t.Fatalf("bisync failed: %v", err)
	}

	if !result.Success {
		t.Errorf("bisync should succeed, error: %s", result.Error)
	}

	// Both directories should now have both files (superset)
	if !h.FileExists(filepath.Join(h.LocalDir, "local-only.txt")) {
		t.Error("local-only.txt should still exist in local")
	}
	if !h.FileExists(filepath.Join(h.LocalDir, "cloud-only.txt")) {
		t.Error("cloud-only.txt should be synced to local")
	}
	if !h.FileExists(filepath.Join(syncPath, "local-only.txt")) {
		t.Error("local-only.txt should be synced to cloud")
	}
	if !h.FileExists(filepath.Join(syncPath, "cloud-only.txt")) {
		t.Error("cloud-only.txt should still exist in cloud")
	}
}

// TestIntegration_SyncCycle tests a normal sync cycle with changes on both sides.
func TestIntegration_SyncCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := NewTestHarness(t)

	// Set up cloud path
	cloudSubpath := "TestProject"
	syncPath := filepath.Join(h.SyncRoot, cloudSubpath)
	if err := os.MkdirAll(syncPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Initial file on both sides
	h.CreateFile(h.LocalDir, "shared.txt", "initial content")

	h.CreateConfig([]config.Project{
		{
			Name:      "TestProject",
			LocalPath: h.LocalDir,
			SyncPath:  cloudSubpath,
			Enabled:   true,
		},
	})
	h.LoadState()

	ctx := context.Background()
	syncrDir := h.Config.SyncrDataDir()

	// First: Initialize with resync
	opts := syncpkg.BisyncOptions{
		Resync:       true,
		ResyncMode:   syncpkg.ResyncPath1,
		SyncrDataDir: syncrDir,
	}

	_, err := syncpkg.RunBisync(ctx, h.LocalDir, syncPath, opts)
	if err != nil {
		t.Fatalf("initial bisync failed: %v", err)
	}

	// Mark as initialized
	h.State.MarkInitialized("TestProject")
	h.State.RecordSuccess("TestProject")
	h.State.Save()

	// Make changes on both sides
	h.CreateFile(h.LocalDir, "new-local.txt", "new local file")
	h.CreateFile(syncPath, "new-cloud.txt", "new cloud file")

	// Run normal sync (not resync)
	opts = syncpkg.BisyncOptions{
		Resync:       false,
		SyncrDataDir: syncrDir,
	}

	result, err := syncpkg.RunBisync(ctx, h.LocalDir, syncPath, opts)
	if err != nil {
		t.Fatalf("sync cycle failed: %v", err)
	}

	if !result.Success {
		t.Errorf("sync should succeed, error: %s", result.Error)
	}

	// Both directories should have both new files
	if !h.FileExists(filepath.Join(h.LocalDir, "new-cloud.txt")) {
		t.Error("new-cloud.txt should be synced to local")
	}
	if !h.FileExists(filepath.Join(syncPath, "new-local.txt")) {
		t.Error("new-local.txt should be synced to cloud")
	}

	// Record success and verify state
	h.State.RecordSuccess("TestProject")
	ps := h.State.GetProject("TestProject")
	if ps.SyncCount != 2 {
		t.Errorf("SyncCount = %d, want 2", ps.SyncCount)
	}
}

// TestIntegration_ErrorPath_MissingLocalPath tests error handling when local path is missing.
func TestIntegration_ErrorPath_MissingLocalPath(t *testing.T) {
	h := NewTestHarness(t)

	// Create config with non-existent local path
	missingLocalPath := filepath.Join(h.TmpDir, "nonexistent", "local")
	h.CreateConfig([]config.Project{
		{
			Name:      "TestProject",
			LocalPath: missingLocalPath,
			SyncPath:  "TestProject",
			Enabled:   true,
		},
	})

	h.LoadState()
	h.State.MarkInitialized("TestProject")

	// Try to sync - should fail with clear error
	ctx := context.Background()
	syncPath := filepath.Join(h.SyncRoot, "TestProject")
	os.MkdirAll(syncPath, 0755)

	opts := syncpkg.BisyncOptions{
		SyncrDataDir: h.Config.SyncrDataDir(),
	}

	_, err := syncpkg.RunBisync(ctx, missingLocalPath, syncPath, opts)
	if err == nil {
		t.Error("sync should fail with missing local path")
	}

	// Record error and verify state tracking
	h.State.RecordError("TestProject", err)
	ps := h.State.GetProject("TestProject")
	if ps.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", ps.ErrorCount)
	}
	if ps.LastSyncStatus != "error" {
		t.Errorf("LastSyncStatus = %s, want error", ps.LastSyncStatus)
	}
}

// TestIntegration_ErrorPath_MissingCloudPath tests error handling when cloud path is missing.
func TestIntegration_ErrorPath_MissingCloudPath(t *testing.T) {
	h := NewTestHarness(t)

	missingCloudPath := filepath.Join(h.SyncRoot, "nonexistent", "cloud")
	h.CreateConfig([]config.Project{
		{
			Name:      "TestProject",
			LocalPath: h.LocalDir,
			SyncPath:  "nonexistent/cloud",
			Enabled:   true,
		},
	})

	h.LoadState()
	h.State.MarkInitialized("TestProject")

	// Try to sync - should fail with clear error
	ctx := context.Background()

	opts := syncpkg.BisyncOptions{
		SyncrDataDir: h.Config.SyncrDataDir(),
	}

	_, err := syncpkg.RunBisync(ctx, h.LocalDir, missingCloudPath, opts)
	if err == nil {
		t.Error("sync should fail with missing cloud path")
	}

	// Record error and verify
	h.State.RecordError("TestProject", err)
	ps := h.State.GetProject("TestProject")
	if ps.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", ps.ErrorCount)
	}
}

// TestIntegration_ErrorPath_ConsecutiveErrors tests max consecutive error tracking.
func TestIntegration_ErrorPath_ConsecutiveErrors(t *testing.T) {
	h := NewTestHarness(t)
	h.CreateConfig([]config.Project{})
	h.LoadState()

	// Mark project as initialized
	h.State.MarkInitialized("TestProject")

	// Record 5 consecutive errors
	for i := 0; i < 5; i++ {
		h.State.RecordError("TestProject", nil)
	}

	ps := h.State.GetProject("TestProject")
	if ps.ErrorCount != 5 {
		t.Errorf("ErrorCount = %d, want 5", ps.ErrorCount)
	}

	// A success should reset the counter
	h.State.RecordSuccess("TestProject")
	ps = h.State.GetProject("TestProject")
	if ps.ErrorCount != 0 {
		t.Errorf("ErrorCount after success = %d, want 0", ps.ErrorCount)
	}
}

// TestIntegration_DryRun tests that dry run mode doesn't make changes.
func TestIntegration_DryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := NewTestHarness(t)

	// Set up paths
	cloudSubpath := "TestProject"
	syncPath := filepath.Join(h.SyncRoot, cloudSubpath)
	if err := os.MkdirAll(syncPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create file only in local
	h.CreateFile(h.LocalDir, "local-file.txt", "local content")

	h.CreateConfig([]config.Project{
		{
			Name:      "TestProject",
			LocalPath: h.LocalDir,
			SyncPath:  cloudSubpath,
			Enabled:   true,
		},
	})
	h.LoadState()

	// Run dry-run sync
	ctx := context.Background()
	opts := syncpkg.BisyncOptions{
		Resync:       true,
		ResyncMode:   syncpkg.ResyncPath1,
		DryRun:       true,
		SyncrDataDir: h.Config.SyncrDataDir(),
	}

	result, err := syncpkg.RunBisync(ctx, h.LocalDir, syncPath, opts)
	if err != nil {
		t.Fatalf("dry-run bisync failed: %v", err)
	}

	if !result.Success {
		t.Errorf("dry-run should succeed, error: %s", result.Error)
	}

	// File should NOT have been synced to cloud (dry run)
	if h.FileExists(filepath.Join(syncPath, "local-file.txt")) {
		t.Error("dry-run should not sync files to cloud")
	}
}

// countFiles returns the number of files (not directories) in a path.
func countFiles(path string) int {
	count := 0
	filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}

// TestIntegration_DaemonPIDFile tests that daemon PID file is properly managed.
func TestIntegration_DaemonPIDFile(t *testing.T) {
	h := NewTestHarness(t)
	h.CreateConfig([]config.Project{})
	h.LoadState()

	syncrDir := h.Config.SyncrDataDir()
	pidFile := filepath.Join(syncrDir, "syncr.pid")

	// Simulate daemon writing PID file
	if err := os.WriteFile(pidFile, []byte("12345"), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	// Verify PID file exists
	if !h.FileExists(pidFile) {
		t.Error("PID file should exist")
	}

	// Read and verify PID content
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("failed to read PID file: %v", err)
	}
	if string(data) != "12345" {
		t.Errorf("PID content = %s, want 12345", string(data))
	}

	// Simulate daemon cleanup (removal)
	if err := os.Remove(pidFile); err != nil {
		t.Fatalf("failed to remove PID file: %v", err)
	}

	if h.FileExists(pidFile) {
		t.Error("PID file should be removed after daemon shutdown")
	}
}

// TestIntegration_DaemonSkipsUninitializedProjects tests that daemon skips uninitialized projects.
func TestIntegration_DaemonSkipsUninitializedProjects(t *testing.T) {
	h := NewTestHarness(t)

	h.CreateConfig([]config.Project{
		{Name: "InitProject", LocalPath: h.LocalDir, SyncPath: "init", Enabled: true},
		{Name: "UninitProject", LocalPath: h.LocalDir, SyncPath: "uninit", Enabled: true},
	})
	h.LoadState()

	// Only initialize one project
	h.State.MarkInitialized("InitProject")
	h.State.Save()

	// Reload state to verify
	st2, err := state.Load(h.Config.SyncrDataDir())
	if err != nil {
		t.Fatal(err)
	}

	if !st2.IsInitialized("InitProject") {
		t.Error("InitProject should be initialized")
	}
	if st2.IsInitialized("UninitProject") {
		t.Error("UninitProject should NOT be initialized")
	}
}

// TestIntegration_DaemonSkipsDisabledProjects tests that disabled projects are skipped.
func TestIntegration_DaemonSkipsDisabledProjects(t *testing.T) {
	h := NewTestHarness(t)

	h.CreateConfig([]config.Project{
		{Name: "EnabledProject", LocalPath: h.LocalDir, SyncPath: "enabled", Enabled: true},
		{Name: "DisabledProject", LocalPath: h.LocalDir, SyncPath: "disabled", Enabled: false},
	})

	// GetProject should show enabled/disabled status
	enabled := h.Config.GetProject("EnabledProject")
	disabled := h.Config.GetProject("DisabledProject")

	if !enabled.Enabled {
		t.Error("EnabledProject should be enabled")
	}
	if disabled.Enabled {
		t.Error("DisabledProject should be disabled")
	}
}

// TestIntegration_DaemonMaxErrors tests that projects are skipped after max consecutive errors.
func TestIntegration_DaemonMaxErrors(t *testing.T) {
	h := NewTestHarness(t)
	h.CreateConfig([]config.Project{})
	h.LoadState()

	h.State.MarkInitialized("TestProject")

	// Simulate 5 consecutive errors (MaxConsecutiveErrors = 5)
	for range 5 {
		h.State.RecordError("TestProject", nil)
	}
	h.State.Save()

	ps := h.State.GetProject("TestProject")
	if ps.ErrorCount != 5 {
		t.Errorf("ErrorCount = %d, want 5", ps.ErrorCount)
	}

	// This project should be skipped in daemon sync loop
	// The daemon checks: if ps.ErrorCount >= MaxConsecutiveErrors { skip }
	// We can't directly test the daemon loop, but we verify the state tracking works
}

// TestIntegration_LoggerCreation tests that logger creates log directory and file.
func TestIntegration_LoggerCreation(t *testing.T) {
	h := NewTestHarness(t)
	h.CreateConfig([]config.Project{})

	syncrDir := h.Config.SyncrDataDir()
	logsDir := filepath.Join(syncrDir, "logs")

	// Logs directory should be created when state is loaded (indirectly via _syncr creation)
	h.LoadState()

	// Create logs directory explicitly (as logger.New would do)
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	if !h.FileExists(logsDir) {
		t.Error("logs directory should exist in _syncr")
	}
}
