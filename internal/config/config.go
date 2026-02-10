// Package config handles loading, validation, and saving of syncr configuration.
// Configuration is stored in syncr.json in the working directory or a custom path.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ValidationResult holds categorized validation results.
type ValidationResult struct {
	Errors   []string // Fatal issues that prevent syncing
	Warnings []string // Non-fatal issues the user should be aware of
}

// OK returns true if there are no errors (warnings are acceptable).
func (v ValidationResult) OK() bool {
	return len(v.Errors) == 0
}

// HasIssues returns true if there are any errors or warnings.
func (v ValidationResult) HasIssues() bool {
	return len(v.Errors) > 0 || len(v.Warnings) > 0
}

// Config represents the syncr configuration.
type Config struct {
	SyncRoot            string    `json:"sync_root"`
	SyncIntervalSeconds int       `json:"sync_interval_seconds"`
	Projects            []Project `json:"projects"`

	path         string // file path (not serialized)
	localDataDir string // resolved local data directory (not serialized)
}

// Project represents a single sync project.
type Project struct {
	Name      string `json:"name"`
	LocalPath string `json:"local_path"`
	SyncPath  string `json:"sync_path"`
	Enabled   bool   `json:"enabled"`
}

// Load loads configuration from the specified path or default location.
// Search order:
// 1. Explicit configPath (if provided)
// 2. ./syncr.json (current working directory)
func Load(configPath string) (*Config, error) {
	path := configPath
	if path == "" {
		path = "syncr.json"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Store the path for later reference
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	cfg.path = absPath

	// Apply defaults
	if cfg.SyncIntervalSeconds == 0 {
		cfg.SyncIntervalSeconds = 300 // 5 minutes
	}

	// Resolve local data directory for per-machine working files
	if stateDir, err := StateDir(); err == nil {
		cfg.localDataDir = stateDir
	}

	return &cfg, nil
}

// Save writes the configuration to disk.
func (c *Config) Save() error {
	if c.path == "" {
		return errors.New("config path not set")
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.SyncRoot == "" {
		return errors.New("sync_root is required")
	}

	if !filepath.IsAbs(c.SyncRoot) {
		return errors.New("sync_root must be an absolute path")
	}

	if _, err := os.Stat(c.SyncRoot); os.IsNotExist(err) {
		return fmt.Errorf("sync_root does not exist: %s", c.SyncRoot)
	}

	if c.SyncIntervalSeconds < 60 {
		return errors.New("sync_interval_seconds must be at least 60")
	}

	// Check for duplicate project names and overlapping sync paths
	names := make(map[string]bool)
	syncPaths := make([]string, 0, len(c.Projects))

	for _, p := range c.Projects {
		if p.Name == "" {
			return errors.New("project name is required")
		}
		if names[p.Name] {
			return fmt.Errorf("duplicate project name: %s", p.Name)
		}
		names[p.Name] = true

		if !filepath.IsAbs(p.LocalPath) {
			return fmt.Errorf("project %s: local_path must be an absolute path", p.Name)
		}

		// Validate sync_path: check for duplicates and overlapping paths
		normalized := filepath.Clean(p.SyncPath)
		if normalized == "" || normalized == "." {
			normalized = p.Name // Default to project name if empty
		}

		for _, existing := range syncPaths {
			if normalized == existing {
				return fmt.Errorf("duplicate sync_path: %s", p.SyncPath)
			}
			// Check for parent/child overlap (nested paths)
			if isOverlappingPath(normalized, existing) {
				return fmt.Errorf("overlapping sync_path: %q and %q would conflict", p.SyncPath, existing)
			}
		}
		syncPaths = append(syncPaths, normalized)
	}

	return nil
}

// isOverlappingPath checks if two paths overlap (one is parent of the other).
func isOverlappingPath(path1, path2 string) bool {
	// Normalize paths and add separator for proper prefix matching
	p1 := filepath.Clean(path1) + string(filepath.Separator)
	p2 := filepath.Clean(path2) + string(filepath.Separator)

	// Check if either is a prefix of the other
	return len(p1) != len(p2) && (hasPathPrefix(p1, p2) || hasPathPrefix(p2, p1))
}

// hasPathPrefix checks if path starts with prefix (using path separators).
func hasPathPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

// isDirWritable checks if a directory is writable by creating and removing a temp file.
func isDirWritable(path string) bool {
	tmp := filepath.Join(path, ".syncr_write_test")
	f, err := os.Create(tmp)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(tmp)
	return true
}

// ValidateFull performs comprehensive validation, collecting all issues rather
// than returning on the first error. Structural errors are checked first,
// then filesystem warnings. The existing Validate() method is unchanged.
func (c *Config) ValidateFull() ValidationResult {
	var result ValidationResult

	// Structural errors
	if c.SyncRoot == "" {
		result.Errors = append(result.Errors, "sync_root is required")
	} else if !filepath.IsAbs(c.SyncRoot) {
		result.Errors = append(result.Errors, "sync_root must be an absolute path")
	}

	if c.SyncIntervalSeconds < 60 {
		result.Errors = append(result.Errors, "sync_interval_seconds must be at least 60")
	}

	names := make(map[string]bool)
	syncPaths := make([]string, 0, len(c.Projects))
	enabledCount := 0

	for _, p := range c.Projects {
		if p.Name == "" {
			result.Errors = append(result.Errors, "project name is required")
			continue
		}
		if names[p.Name] {
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate project name: %s", p.Name))
			continue
		}
		names[p.Name] = true

		if p.Enabled {
			enabledCount++
		}

		if p.LocalPath == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("project %s: local_path is required", p.Name))
		} else if !filepath.IsAbs(p.LocalPath) {
			result.Errors = append(result.Errors, fmt.Sprintf("project %s: local_path must be an absolute path", p.Name))
		}

		// Check sync_path duplicates and overlaps
		normalized := filepath.Clean(p.SyncPath)
		if normalized == "" || normalized == "." {
			normalized = p.Name
		}

		for _, existing := range syncPaths {
			if normalized == existing {
				result.Errors = append(result.Errors, fmt.Sprintf("duplicate sync_path: %s", p.SyncPath))
				break
			}
			if isOverlappingPath(normalized, existing) {
				result.Errors = append(result.Errors, fmt.Sprintf("overlapping sync_path: %q and %q would conflict", p.SyncPath, existing))
				break
			}
		}
		syncPaths = append(syncPaths, normalized)
	}

	// Filesystem warnings (only if sync_root is a valid absolute path)
	if c.SyncRoot != "" && filepath.IsAbs(c.SyncRoot) {
		if info, err := os.Stat(c.SyncRoot); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("sync_root directory does not exist: %s", c.SyncRoot))
		} else if err == nil && info.IsDir() && !isDirWritable(c.SyncRoot) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("sync_root directory is not writable: %s", c.SyncRoot))
		}
	}

	for _, p := range c.Projects {
		if p.Name == "" || p.LocalPath == "" || !filepath.IsAbs(p.LocalPath) {
			continue
		}
		if _, err := os.Stat(p.LocalPath); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("project %s: local_path does not exist: %s", p.Name, p.LocalPath))
		}

		// Check cloud path existence
		if c.SyncRoot != "" && filepath.IsAbs(c.SyncRoot) {
			syncPath := filepath.Clean(p.SyncPath)
			if syncPath == "" || syncPath == "." {
				syncPath = p.Name
			}
			cloudPath := filepath.Join(c.SyncRoot, syncPath)
			if _, err := os.Stat(cloudPath); os.IsNotExist(err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("project %s: cloud path does not exist: %s", p.Name, cloudPath))
			}
		}
	}

	if len(c.Projects) > 0 && enabledCount == 0 {
		result.Warnings = append(result.Warnings, "no enabled projects")
	}

	return result
}

// Path returns the configuration file path.
func (c *Config) Path() string {
	return c.path
}

// StateDir returns the local, per-machine directory for state storage.
// Uses os.UserConfigDir() so state does not travel with cloud-synced data.
func StateDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determining config directory: %w", err)
	}
	return filepath.Join(configDir, "syncr"), nil
}

// SyncrDataDir returns the local per-machine directory for working files
// (bisync state, logs, PID file). Falls back to {sync_root}/_syncr if the
// local directory was not resolved.
func (c *Config) SyncrDataDir() string {
	if c.localDataDir != "" {
		return c.localDataDir
	}
	// Fallback for manually constructed configs (e.g. tests)
	return filepath.Join(c.SyncRoot, "_syncr")
}

// SetLocalDataDir overrides the local data directory.
// This is primarily for testing.
func (c *Config) SetLocalDataDir(path string) {
	c.localDataDir = path
}

// GetProject returns a project by name, or nil if not found.
func (c *Config) GetProject(name string) *Project {
	for i := range c.Projects {
		if c.Projects[i].Name == name {
			return &c.Projects[i]
		}
	}
	return nil
}

// DefaultConfig returns a default configuration template.
func DefaultConfig() *Config {
	return &Config{
		SyncRoot:            "",
		SyncIntervalSeconds: 300,
		Projects:            []Project{},
	}
}
