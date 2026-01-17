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
	CloudRoot           string    `json:"cloud_root"`
	SyncIntervalSeconds int       `json:"sync_interval_seconds"`
	Projects            []Project `json:"projects"`

	path string // file path (not serialized)
}

// Project represents a single sync project.
type Project struct {
	Name         string `json:"name"`
	LocalPath    string `json:"local_path"`
	CloudSubpath string `json:"cloud_subpath"`
	Enabled      bool   `json:"enabled"`
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
	if c.CloudRoot == "" {
		return errors.New("cloud_root is required")
	}

	if !filepath.IsAbs(c.CloudRoot) {
		return errors.New("cloud_root must be an absolute path")
	}

	if _, err := os.Stat(c.CloudRoot); os.IsNotExist(err) {
		return fmt.Errorf("cloud_root does not exist: %s", c.CloudRoot)
	}

	if c.SyncIntervalSeconds < 60 {
		return errors.New("sync_interval_seconds must be at least 60")
	}

	// Check for duplicate project names
	names := make(map[string]bool)
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
	}

	return nil
}

// Path returns the configuration file path.
func (c *Config) Path() string {
	return c.path
}

// SyncrDataDir returns the path to the _syncr metadata directory.
func (c *Config) SyncrDataDir() string {
	return filepath.Join(c.CloudRoot, "_syncr")
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
		CloudRoot:           "",
		SyncIntervalSeconds: 300,
		Projects:            []Project{},
	}
}
