// Package state manages sync state tracking for projects.
// State is stored in {sync_root}/_syncr/state.json so it travels with the data
// and is accessible from any machine.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State tracks project initialization and sync history.
// It is stored at {sync_root}/_syncr/state.json so it travels with the data.
type State struct {
	Version   int                     `json:"version"`
	MachineID string                  `json:"machine_id"`
	Projects  map[string]ProjectState `json:"projects"`

	path string       // file path (not serialized)
	mu   sync.RWMutex // thread safety
}

// ProjectState tracks the state of a single project.
type ProjectState struct {
	Initialized    bool      `json:"initialized"`
	InitializedAt  time.Time `json:"initialized_at,omitempty"`
	LastSync       time.Time `json:"last_sync,omitempty"`
	LastSyncStatus string    `json:"last_sync_status,omitempty"` // "success", "error", "conflicts"
	SyncCount      int       `json:"sync_count"`
	ErrorCount     int       `json:"error_count"`
	LastError      string    `json:"last_error,omitempty"`
}

const stateVersion = 1

// Load loads state from the _syncr directory.
// Creates the directory and an empty state file if they don't exist.
func Load(syncrDataDir string) (*State, error) {
	// Ensure _syncr directory exists
	if err := os.MkdirAll(syncrDataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating syncr data dir: %w", err)
	}

	statePath := filepath.Join(syncrDataDir, "state.json")

	// Get machine ID
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Try to load existing state
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new state
			s := &State{
				Version:   stateVersion,
				MachineID: hostname,
				Projects:  make(map[string]ProjectState),
				path:      statePath,
			}
			return s, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	s.path = statePath
	s.MachineID = hostname // Always use current hostname

	if s.Projects == nil {
		s.Projects = make(map[string]ProjectState)
	}

	return &s, nil
}

// Save writes the state to disk atomically.
func (s *State) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		return fmt.Errorf("state path not set")
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding state: %w", err)
	}

	// Write to temp file first, then rename (atomic)
	dir := filepath.Dir(s.path)
	tmpFile, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// GetProject returns the state for a project (copy, not reference).
func (s *State) GetProject(name string) ProjectState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Projects[name]
}

// IsInitialized returns true if the project has been initialized.
func (s *State) IsInitialized(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Projects[name].Initialized
}

// MarkInitialized marks a project as initialized.
func (s *State) MarkInitialized(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ps := s.Projects[name]
	ps.Initialized = true
	ps.InitializedAt = time.Now().UTC()
	s.Projects[name] = ps

	return nil
}

// RecordSuccess records a successful sync.
func (s *State) RecordSuccess(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ps := s.Projects[name]
	ps.LastSync = time.Now().UTC()
	ps.LastSyncStatus = "success"
	ps.SyncCount++
	ps.ErrorCount = 0 // Reset consecutive error count
	ps.LastError = ""
	s.Projects[name] = ps
}

// RecordConflicts records a sync that completed with conflicts.
func (s *State) RecordConflicts(name string, conflictCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ps := s.Projects[name]
	ps.LastSync = time.Now().UTC()
	ps.LastSyncStatus = "conflicts"
	ps.SyncCount++
	ps.ErrorCount = 0 // Conflicts aren't errors
	ps.LastError = fmt.Sprintf("%d conflicts", conflictCount)
	s.Projects[name] = ps
}

// RecordError records a failed sync.
func (s *State) RecordError(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ps := s.Projects[name]
	ps.LastSyncStatus = "error"
	ps.ErrorCount++
	if err != nil {
		ps.LastError = err.Error()
	}
	s.Projects[name] = ps
}

// Path returns the state file path.
func (s *State) Path() string {
	return s.path
}
