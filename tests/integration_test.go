package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/state"
	"github.com/peteretelej/syncr/internal/sync"
)

// TestHarness provides a temporary environment for integration tests.
type TestHarness struct {
	TmpDir     string
	CloudRoot  string
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
		CloudRoot:  filepath.Join(tmpDir, "cloud"),
		LocalDir:   filepath.Join(tmpDir, "local"),
		ConfigPath: filepath.Join(tmpDir, "syncr.json"),
		t:          t,
	}

	// Create directories
	if err := os.MkdirAll(h.CloudRoot, 0755); err != nil {
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
		CloudRoot:           h.CloudRoot,
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
			Name:         "TestProject",
			LocalPath:    h.LocalDir,
			CloudSubpath: "TestProject",
			Enabled:      true,
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
	count, err := sync.CountConflicts(h.LocalDir)
	if err != nil {
		t.Fatalf("CountConflicts failed: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 conflicts, got %d", count)
	}

	// List conflicts
	conflicts, err := sync.ListConflicts(h.LocalDir)
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
		{Name: "ProjectA", LocalPath: localA, CloudSubpath: "A", Enabled: true},
		{Name: "ProjectB", LocalPath: localB, CloudSubpath: "B", Enabled: true},
		{Name: "ProjectC", LocalPath: localA, CloudSubpath: "C", Enabled: false},
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
