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

// Config represents the syncr configuration.
type Config struct {
	SyncRoot            string    `json:"sync_root"`
	SyncIntervalSeconds int       `json:"sync_interval_seconds"`
	Projects            []Project `json:"projects"`

	path string // file path (not serialized)
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

// SyncrDataDir returns the path to the _syncr metadata directory.
func (c *Config) SyncrDataDir() string {
	return filepath.Join(c.SyncRoot, "_syncr")
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
