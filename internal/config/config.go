// Package config handles loading, validation, and saving of syncr configuration.
// Configuration is stored in syncr.json in the working directory or a custom path.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// validConflictResolve defines the accepted values for conflict_resolve.
var validConflictResolve = map[string]bool{
	"none":    true,
	"newer":   true,
	"older":   true,
	"larger":  true,
	"smaller": true,
	"path1":   true,
	"path2":   true,
}

// Config represents the syncr configuration.
type Config struct {
	SyncRoot                string    `json:"sync_root"`
	SyncIntervalMinutes     int       `json:"sync_interval_minutes"`
	BackupRetentionDaysJSON *int      `json:"backup_retention_days,omitempty"`
	ConflictResolve         string    `json:"conflict_resolve,omitempty"`
	ConflictSuffix          string    `json:"conflict_suffix,omitempty"`
	Exclude                 []string  `json:"exclude,omitempty"`
	Projects                []Project `json:"projects"`

	path                string // file path (not serialized)
	localDataDir        string // resolved local data directory (not serialized)
	backupRetentionDays int    // resolved value (not serialized)
}

// Hooks defines shell commands to run after sync events.
type Hooks struct {
	PostSync   string `json:"post_sync,omitempty"`
	OnConflict string `json:"on_conflict,omitempty"`
}

// Project represents a single sync project.
type Project struct {
	Name               string            `json:"name"`
	LocalPath          string            `json:"local_path"`
	SyncPath           string            `json:"sync_path"`
	Enabled            bool              `json:"enabled"`
	Exclude            []string          `json:"exclude,omitempty"`
	Hooks              *Hooks            `json:"hooks,omitempty"`
	HookTimeoutSeconds int               `json:"hook_timeout_seconds,omitempty"`
	Derived            map[string]string `json:"derived,omitempty"`
	BackupDir          bool              `json:"backup_dir,omitempty"`
	ConflictResolve    string            `json:"conflict_resolve,omitempty"`
}

// Load loads configuration from the specified path or default location.
// Search order:
// 1. Explicit configPath (if provided)
// 2. SYNCR_CONFIG environment variable
// 3. ./syncr.json (current working directory)
func Load(configPath string) (*Config, error) {
	path := configPath
	if path == "" {
		path = os.Getenv("SYNCR_CONFIG")
	}
	if path == "" {
		path = "syncr.json"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s (run 'syncr init' to create one, or set SYNCR_CONFIG)", path)
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
	if cfg.SyncIntervalMinutes == 0 {
		cfg.SyncIntervalMinutes = 5 // 5 minutes
	}

	// Resolve local data directory for per-machine working files
	dataDir, err := dataDir()
	if err != nil {
		return nil, fmt.Errorf("resolving data directory: %w", err)
	}
	cfg.localDataDir = dataDir

	// Resolve backup retention days: nil pointer means use default (30)
	if cfg.BackupRetentionDaysJSON != nil {
		cfg.backupRetentionDays = *cfg.BackupRetentionDaysJSON
	} else {
		cfg.backupRetentionDays = 30
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

	if c.SyncIntervalMinutes < 1 {
		return errors.New("sync_interval_minutes must be at least 1")
	}

	for _, pattern := range c.Exclude {
		if pattern == "" {
			return errors.New("empty string in global exclude list")
		}
	}

	// Validate global conflict_resolve
	if c.ConflictResolve != "" && !validConflictResolve[c.ConflictResolve] {
		return fmt.Errorf("invalid conflict_resolve value: %q", c.ConflictResolve)
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

		// Validate exclude patterns
		for _, pattern := range p.Exclude {
			if pattern == "" {
				return fmt.Errorf("project %s: empty string in exclude list", p.Name)
			}
		}

		// Validate hook_timeout_seconds
		if p.HookTimeoutSeconds < 0 {
			// Negative treated as default; warn via ValidateFull only
		}

		// Validate conflict_resolve
		if p.ConflictResolve != "" && !validConflictResolve[p.ConflictResolve] {
			return fmt.Errorf("project %s: invalid conflict_resolve value: %q", p.Name, p.ConflictResolve)
		}

		// Validate sync_path: check for duplicates and overlapping paths
		normalized := filepath.Clean(p.SyncPath)
		if normalized == "" || normalized == "." {
			normalized = p.Name // Default to project name if empty
		}

		// Reject reserved _syncr path
		if normalized == "_syncr" || strings.HasPrefix(normalized, "_syncr"+string(filepath.Separator)) {
			return fmt.Errorf("project %s: sync_path %q conflicts with reserved _syncr directory", p.Name, p.SyncPath)
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

	if c.SyncIntervalMinutes < 1 {
		result.Errors = append(result.Errors, "sync_interval_minutes must be at least 1")
	}

	for _, pattern := range c.Exclude {
		if pattern == "" {
			result.Errors = append(result.Errors, "empty string in global exclude list")
		} else if pattern == "*" || pattern == "**" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("global exclude pattern %q would exclude all files", pattern))
		}
	}

	// Validate global conflict_resolve
	if c.ConflictResolve != "" {
		if !validConflictResolve[c.ConflictResolve] {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid conflict_resolve value: %q", c.ConflictResolve))
		} else if c.ConflictResolve == "newer" || c.ConflictResolve == "older" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("conflict_resolve %q depends on modtime accuracy; may silently fall back to none", c.ConflictResolve))
		}
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

		// Validate exclude patterns
		for _, pattern := range p.Exclude {
			if pattern == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("project %s: empty string in exclude list", p.Name))
			} else if pattern == "*" || pattern == "**" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("project %s: exclude pattern %q would exclude all files", p.Name, pattern))
			}
		}

		// Validate hook_timeout_seconds
		if p.HookTimeoutSeconds < 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("project %s: negative hook_timeout_seconds (%d), using default (30s)", p.Name, p.HookTimeoutSeconds))
		}

		// Validate conflict_resolve
		if p.ConflictResolve != "" {
			if !validConflictResolve[p.ConflictResolve] {
				result.Errors = append(result.Errors, fmt.Sprintf("project %s: invalid conflict_resolve value: %q", p.Name, p.ConflictResolve))
			} else if p.ConflictResolve == "newer" || p.ConflictResolve == "older" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("project %s: conflict_resolve %q depends on modtime accuracy; may silently fall back to none", p.Name, p.ConflictResolve))
			}
		}

		// Check sync_path duplicates and overlaps
		normalized := filepath.Clean(p.SyncPath)
		if normalized == "" || normalized == "." {
			normalized = p.Name
		}

		// Reject reserved _syncr path
		if normalized == "_syncr" || strings.HasPrefix(normalized, "_syncr"+string(filepath.Separator)) {
			result.Errors = append(result.Errors, fmt.Sprintf("project %s: sync_path %q conflicts with reserved _syncr directory", p.Name, p.SyncPath))
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

	// Warn on negative backup retention
	if c.backupRetentionDays < 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("backup_retention_days is negative (%d), trash cleanup disabled", c.backupRetentionDays))
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

		// Check sync folder existence
		if c.SyncRoot != "" && filepath.IsAbs(c.SyncRoot) {
			syncPath := filepath.Clean(p.SyncPath)
			if syncPath == "" || syncPath == "." {
				syncPath = p.Name
			}
			fullSyncPath := filepath.Join(c.SyncRoot, syncPath)
			if _, err := os.Stat(fullSyncPath); os.IsNotExist(err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("project %s: sync folder does not exist: %s", p.Name, fullSyncPath))
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

// dataDir returns the local, per-machine directory for syncr data
// (state, logs, bisync working files, PID). Uses ~/.config/syncr which
// resolves reliably across platforms, including service/daemon contexts
// where OS-specific config directories may not be available.
func dataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".config", "syncr"), nil
}

// SyncrDataDir returns the local per-machine directory for working files
// (bisync state, logs, PID file). Load() guarantees localDataDir is set;
// panics if called on a Config where it was never resolved (programming error).
func (c *Config) SyncrDataDir() string {
	if c.localDataDir == "" {
		panic("syncr: SyncrDataDir called before localDataDir was set (use Load() or SetLocalDataDir())")
	}
	return c.localDataDir
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

// BackupRetentionDays returns the resolved backup retention period in days.
// Defaults to 30 if not set in the config file.
func (c *Config) BackupRetentionDays() int {
	return c.backupRetentionDays
}

// ResolvedExcludes returns the combined global and project exclude patterns.
func (c *Config) ResolvedExcludes(projectName string) []string {
	excludes := make([]string, 0, len(c.Exclude))
	excludes = append(excludes, c.Exclude...)
	if p := c.GetProject(projectName); p != nil {
		excludes = append(excludes, p.Exclude...)
	}
	return excludes
}

// ResolvedConflictResolve returns the effective conflict resolution strategy
// for the named project. Project-level overrides take precedence over the
// global setting. Returns "" when no strategy is configured.
func (c *Config) ResolvedConflictResolve(projectName string) string {
	if p := c.GetProject(projectName); p != nil && p.ConflictResolve != "" {
		return p.ConflictResolve
	}
	return c.ConflictResolve
}

// ResolvedConflictSuffix returns the configured conflict suffix, or ""
// if none is set (meaning rclone's default should be used).
func (c *Config) ResolvedConflictSuffix() string {
	return c.ConflictSuffix
}

// TrashDir returns the trash directory path for a given project.
func (c *Config) TrashDir(projectName string) string {
	return filepath.Join(c.SyncRoot, "_syncr", "trash", projectName)
}

// DefaultConfig returns a default configuration template.
func DefaultConfig() *Config {
	return &Config{
		SyncRoot:            "",
		SyncIntervalMinutes: 5,
		Projects:            []Project{},
		backupRetentionDays: 30,
	}
}
