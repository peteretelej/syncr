package state

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoad_NewState(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	s, err := Load(syncrDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if s.Version != stateVersion {
		t.Errorf("Version = %d, want %d", s.Version, stateVersion)
	}

	if s.MachineID == "" {
		t.Error("MachineID is empty")
	}

	if s.Projects == nil {
		t.Error("Projects is nil")
	}

	// Verify _syncr directory was created
	if _, err := os.Stat(syncrDir); os.IsNotExist(err) {
		t.Error("_syncr directory was not created")
	}
}

func TestLoad_ExistingState(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")
	if err := os.MkdirAll(syncrDir, 0755); err != nil {
		t.Fatal(err)
	}

	stateContent := `{
		"version": 1,
		"machine_id": "old-machine",
		"projects": {
			"TestProject": {
				"initialized": true,
				"sync_count": 42
			}
		}
	}`

	statePath := filepath.Join(syncrDir, "state.json")
	if err := os.WriteFile(statePath, []byte(stateContent), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(syncrDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ps := s.GetProject("TestProject")
	if !ps.Initialized {
		t.Error("TestProject should be initialized")
	}
	if ps.SyncCount != 42 {
		t.Errorf("SyncCount = %d, want 42", ps.SyncCount)
	}

	// Machine ID should be updated to current hostname
	if s.MachineID == "old-machine" {
		t.Error("MachineID should be updated to current hostname")
	}
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	s, err := Load(syncrDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Mark a project as initialized
	s.MarkInitialized("TestProject")

	if err := s.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Reload and verify
	s2, err := Load(syncrDir)
	if err != nil {
		t.Fatalf("Load() after save error = %v", err)
	}

	if !s2.IsInitialized("TestProject") {
		t.Error("TestProject should be initialized after reload")
	}
}

func TestMarkInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	s, _ := Load(syncrDir)

	if s.IsInitialized("NewProject") {
		t.Error("NewProject should not be initialized initially")
	}

	s.MarkInitialized("NewProject")

	if !s.IsInitialized("NewProject") {
		t.Error("NewProject should be initialized after MarkInitialized")
	}

	ps := s.GetProject("NewProject")
	if ps.InitializedAt.IsZero() {
		t.Error("InitializedAt should be set")
	}
}

func TestRecordSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	s, _ := Load(syncrDir)

	s.RecordSuccess("TestProject")
	s.RecordSuccess("TestProject")
	s.RecordSuccess("TestProject")

	ps := s.GetProject("TestProject")
	if ps.SyncCount != 3 {
		t.Errorf("SyncCount = %d, want 3", ps.SyncCount)
	}
	if ps.LastSyncStatus != "success" {
		t.Errorf("LastSyncStatus = %q, want %q", ps.LastSyncStatus, "success")
	}
	if ps.LastSync.IsZero() {
		t.Error("LastSync should be set")
	}
}

func TestRecordError(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	s, _ := Load(syncrDir)

	testErr := errors.New("sync failed: network error")
	s.RecordError("TestProject", testErr)
	s.RecordError("TestProject", testErr)

	ps := s.GetProject("TestProject")
	if ps.ErrorCount != 2 {
		t.Errorf("ErrorCount = %d, want 2", ps.ErrorCount)
	}
	if ps.LastError != testErr.Error() {
		t.Errorf("LastError = %q, want %q", ps.LastError, testErr.Error())
	}
}

func TestRecordError_ResetsOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	s, _ := Load(syncrDir)

	// Record some errors
	s.RecordError("TestProject", errors.New("error 1"))
	s.RecordError("TestProject", errors.New("error 2"))

	ps := s.GetProject("TestProject")
	if ps.ErrorCount != 2 {
		t.Fatalf("ErrorCount = %d, want 2", ps.ErrorCount)
	}

	// Record success
	s.RecordSuccess("TestProject")

	ps = s.GetProject("TestProject")
	if ps.ErrorCount != 0 {
		t.Errorf("ErrorCount should reset to 0 after success, got %d", ps.ErrorCount)
	}
	if ps.LastError != "" {
		t.Errorf("LastError should be empty after success, got %q", ps.LastError)
	}
}

func TestRecordConflicts(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	s, _ := Load(syncrDir)

	s.RecordConflicts("TestProject", 3)

	ps := s.GetProject("TestProject")
	if ps.LastSyncStatus != "conflicts" {
		t.Errorf("LastSyncStatus = %q, want %q", ps.LastSyncStatus, "conflicts")
	}
	if ps.SyncCount != 1 {
		t.Errorf("SyncCount = %d, want 1", ps.SyncCount)
	}
	if ps.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0 (conflicts are not errors)", ps.ErrorCount)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	s, _ := Load(syncrDir)

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent writes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.RecordSuccess("TestProject")
		}()
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.GetProject("TestProject")
			_ = s.IsInitialized("TestProject")
		}()
	}

	wg.Wait()

	ps := s.GetProject("TestProject")
	if ps.SyncCount != iterations {
		t.Errorf("SyncCount = %d, want %d", ps.SyncCount, iterations)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")
	if err := os.MkdirAll(syncrDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write invalid JSON
	statePath := filepath.Join(syncrDir, "state.json")
	if err := os.WriteFile(statePath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(syncrDir)
	if err == nil {
		t.Error("Load() expected error for invalid JSON")
	}
}

func TestSave_NoPath(t *testing.T) {
	s := &State{
		Version:   1,
		MachineID: "test",
		Projects:  make(map[string]ProjectState),
		// path is not set
	}

	err := s.Save()
	if err == nil {
		t.Error("Save() expected error when path is not set")
	}
}

func TestPath(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	s := &State{
		path: statePath,
	}

	if got := s.Path(); got != statePath {
		t.Errorf("Path() = %q, want %q", got, statePath)
	}
}

func TestRecordError_NilError(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	s, _ := Load(syncrDir)

	// Record nil error
	s.RecordError("TestProject", nil)

	ps := s.GetProject("TestProject")
	if ps.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", ps.ErrorCount)
	}
	if ps.LastSyncStatus != "error" {
		t.Errorf("LastSyncStatus = %q, want %q", ps.LastSyncStatus, "error")
	}
}

func TestGetProject_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	s, _ := Load(syncrDir)

	ps := s.GetProject("NonExistentProject")
	if ps.Initialized {
		t.Error("Non-existent project should not be initialized")
	}
	if ps.SyncCount != 0 {
		t.Errorf("SyncCount = %d, want 0", ps.SyncCount)
	}
}

func TestLoad_NullProjects(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")
	if err := os.MkdirAll(syncrDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write state with null projects
	stateContent := `{"version": 1, "machine_id": "test", "projects": null}`
	statePath := filepath.Join(syncrDir, "state.json")
	if err := os.WriteFile(statePath, []byte(stateContent), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(syncrDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Projects should be initialized to empty map
	if s.Projects == nil {
		t.Error("Projects should not be nil after loading null")
	}
}
