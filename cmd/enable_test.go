package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// testConfig is a minimal config struct for test verification.
type testConfig struct {
	SyncRoot            string        `json:"sync_root"`
	SyncIntervalSeconds int           `json:"sync_interval_seconds"`
	Projects            []testProject `json:"projects"`
}

type testProject struct {
	Name      string `json:"name"`
	LocalPath string `json:"local_path"`
	SyncPath  string `json:"sync_path"`
	Enabled   bool   `json:"enabled"`
}

// writeTestConfig writes a config file and returns the path.
func writeTestConfig(t *testing.T, dir string, projects []testProject) string {
	t.Helper()
	configPath := filepath.Join(dir, "syncr.json")
	cfg := testConfig{
		SyncRoot:            dir,
		SyncIntervalSeconds: 300,
		Projects:            projects,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

// readTestConfig reads and parses a config file.
func readTestConfig(t *testing.T, path string) testConfig {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg testConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	return cfg
}

func TestSetProjectEnabled_EnableDisabled(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: false},
	})

	SetProjectEnabled([]string{"docs"}, configPath, true)

	cfg := readTestConfig(t, configPath)
	if !cfg.Projects[0].Enabled {
		t.Error("expected project to be enabled after SetProjectEnabled(..., true)")
	}
}

func TestSetProjectEnabled_DisableEnabled(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
	})

	SetProjectEnabled([]string{"docs"}, configPath, false)

	cfg := readTestConfig(t, configPath)
	if cfg.Projects[0].Enabled {
		t.Error("expected project to be disabled after SetProjectEnabled(..., false)")
	}
}

func TestSetProjectEnabled_AlreadyEnabled(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
	})

	// Should not panic or error; just prints "already enabled"
	SetProjectEnabled([]string{"docs"}, configPath, true)

	cfg := readTestConfig(t, configPath)
	if !cfg.Projects[0].Enabled {
		t.Error("project should still be enabled")
	}
}

func TestSetProjectEnabled_AlreadyDisabled(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: false},
	})

	// Should not panic or error; just prints "already disabled"
	SetProjectEnabled([]string{"docs"}, configPath, false)

	cfg := readTestConfig(t, configPath)
	if cfg.Projects[0].Enabled {
		t.Error("project should still be disabled")
	}
}

func TestSetProjectEnabled_MultipleProjects(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
		{Name: "photos", LocalPath: dir, SyncPath: "photos", Enabled: true},
		{Name: "notes", LocalPath: dir, SyncPath: "notes", Enabled: false},
	})

	// Disable only "photos"
	SetProjectEnabled([]string{"photos"}, configPath, false)

	cfg := readTestConfig(t, configPath)
	if !cfg.Projects[0].Enabled {
		t.Error("docs should still be enabled")
	}
	if cfg.Projects[1].Enabled {
		t.Error("photos should be disabled")
	}
	if cfg.Projects[2].Enabled {
		t.Error("notes should still be disabled")
	}
}
