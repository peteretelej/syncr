package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/logger"
)

// writeDaemonTestConfig writes a valid config file and returns the absolute path.
func writeDaemonTestConfig(t *testing.T, dir string, interval int, projects []testProject) string {
	t.Helper()
	configPath := filepath.Join(dir, "syncr.json")
	cfg := testConfig{
		SyncRoot:            dir,
		SyncIntervalSeconds: interval,
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

func TestConfigModTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Missing file should error
	_, err := configModTime(path)
	if err == nil {
		t.Fatal("expected error for missing file")
	}

	// Create file
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	modTime, err := configModTime(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modTime.IsZero() {
		t.Fatal("expected non-zero mod time")
	}

	// Mod time should be recent (within last minute)
	if time.Since(modTime) > time.Minute {
		t.Fatalf("mod time too old: %v", modTime)
	}
}

func TestMaybeReloadConfig_NoChange(t *testing.T) {
	dir := t.TempDir()
	configPath := writeDaemonTestConfig(t, dir, 300, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
	})

	current, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	modTime, err := configModTime(configPath)
	if err != nil {
		t.Fatalf("get mod time: %v", err)
	}

	log := logger.NewStdout(false)

	// Same mod time should return the same config
	got, gotModTime := maybeReloadConfig(configPath, current, modTime, log)
	if got != current {
		t.Error("expected same config pointer when mod time unchanged")
	}
	if !gotModTime.Equal(modTime) {
		t.Error("expected same mod time when unchanged")
	}
}

func TestMaybeReloadConfig_ValidChange(t *testing.T) {
	dir := t.TempDir()
	configPath := writeDaemonTestConfig(t, dir, 300, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
	})

	current, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Use a zero mod time to force reload
	oldModTime := time.Time{}

	log := logger.NewStdout(false)

	got, gotModTime := maybeReloadConfig(configPath, current, oldModTime, log)

	// Should return a new config (different pointer)
	if got == current {
		t.Error("expected new config pointer after reload")
	}
	if len(got.Projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(got.Projects))
	}
	if gotModTime.IsZero() {
		t.Error("expected non-zero mod time after reload")
	}

	// Now add a second project and rewrite
	writeDaemonTestConfig(t, dir, 300, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
		{Name: "photos", LocalPath: dir, SyncPath: "photos", Enabled: true},
	})

	got2, _ := maybeReloadConfig(configPath, got, oldModTime, log)
	if len(got2.Projects) != 2 {
		t.Errorf("expected 2 projects after update, got %d", len(got2.Projects))
	}
}

func TestMaybeReloadConfig_InvalidKeepsOld(t *testing.T) {
	dir := t.TempDir()
	configPath := writeDaemonTestConfig(t, dir, 300, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
	})

	current, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	oldModTime := time.Time{}
	log := logger.NewStdout(false)

	// Write invalid JSON
	if err := os.WriteFile(configPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	got, gotModTime := maybeReloadConfig(configPath, current, oldModTime, log)
	if got != current {
		t.Error("expected original config when new config is invalid JSON")
	}
	// Mod time should be updated to avoid retrying every cycle
	if gotModTime.IsZero() {
		t.Error("expected updated mod time even for invalid config")
	}

	// Write valid JSON but with invalid values (interval too low)
	writeDaemonTestConfig(t, dir, 10, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
	})

	got2, _ := maybeReloadConfig(configPath, current, time.Time{}, log)
	if got2 != current {
		t.Error("expected original config when new config fails validation")
	}
}

func TestMaybeReloadConfig_IntervalChange(t *testing.T) {
	dir := t.TempDir()
	configPath := writeDaemonTestConfig(t, dir, 300, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
	})

	current, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if current.SyncIntervalSeconds != 300 {
		t.Fatalf("expected interval 300, got %d", current.SyncIntervalSeconds)
	}

	// Rewrite config with new interval
	writeDaemonTestConfig(t, dir, 600, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
	})

	log := logger.NewStdout(false)
	got, _ := maybeReloadConfig(configPath, current, time.Time{}, log)

	if got.SyncIntervalSeconds != 600 {
		t.Errorf("expected interval 600 after reload, got %d", got.SyncIntervalSeconds)
	}
}

func TestMaybeReloadConfig_FileDeleted(t *testing.T) {
	dir := t.TempDir()
	configPath := writeDaemonTestConfig(t, dir, 300, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
	})

	current, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	modTime, err := configModTime(configPath)
	if err != nil {
		t.Fatalf("get mod time: %v", err)
	}

	// Delete the config file
	if err := os.Remove(configPath); err != nil {
		t.Fatalf("remove config: %v", err)
	}

	log := logger.NewStdout(false)
	got, gotModTime := maybeReloadConfig(configPath, current, modTime, log)

	if got != current {
		t.Error("expected original config when file is deleted")
	}
	if !gotModTime.Equal(modTime) {
		t.Error("expected unchanged mod time when file is deleted")
	}
}
