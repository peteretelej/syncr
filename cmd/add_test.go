package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/peteretelej/syncr/internal/config"
)

func loadTestConfig(t *testing.T, path string) *config.Config {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func TestAddToConfig_NewProject(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, []testProject{
		{Name: "existing", LocalPath: dir, SyncPath: "existing", Enabled: true},
	})

	cfg := loadTestConfig(t, configPath)

	localDir := filepath.Join(dir, "myproject")
	os.MkdirAll(localDir, 0755)

	err := addToConfig(cfg, "myproject", localDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify project was added
	if len(cfg.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(cfg.Projects))
	}

	p := cfg.GetProject("myproject")
	if p == nil {
		t.Fatal("project 'myproject' not found in config")
	}
	if p.LocalPath != localDir {
		t.Errorf("local_path = %q, want %q", p.LocalPath, localDir)
	}
	if p.SyncPath != "myproject" {
		t.Errorf("sync_path = %q, want %q", p.SyncPath, "myproject")
	}
	if !p.Enabled {
		t.Error("expected project to be enabled")
	}
}

func TestAddToConfig_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, []testProject{
		{Name: "docs", LocalPath: dir, SyncPath: "docs", Enabled: true},
	})

	cfg := loadTestConfig(t, configPath)

	err := addToConfig(cfg, "docs", dir)
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}

	// Config should be unchanged
	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project (unchanged), got %d", len(cfg.Projects))
	}
}

func TestAddToConfig_RelativePathResolved(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, nil)

	cfg := loadTestConfig(t, configPath)

	// Use a relative path
	err := addToConfig(cfg, "reltest", "relative/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := cfg.GetProject("reltest")
	if p == nil {
		t.Fatal("project not found")
	}
	if !filepath.IsAbs(p.LocalPath) {
		t.Errorf("local_path should be absolute, got %q", p.LocalPath)
	}
}

func TestAddToConfig_SyncPathOverlap(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, []testProject{
		{Name: "other", LocalPath: dir, SyncPath: "photos", Enabled: true},
	})

	cfg := loadTestConfig(t, configPath)

	// "photos" as name would produce sync_path "photos", conflicting with existing
	err := addToConfig(cfg, "photos", dir)
	if err == nil {
		t.Fatal("expected error for sync_path overlap, got nil")
	}

	// Config should be unchanged
	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project (unchanged), got %d", len(cfg.Projects))
	}
}

func TestAddToConfig_SavesPersistently(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, []testProject{
		{Name: "existing", LocalPath: dir, SyncPath: "existing", Enabled: true},
	})

	cfg := loadTestConfig(t, configPath)

	localDir := filepath.Join(dir, "newproj")
	os.MkdirAll(localDir, 0755)

	err := addToConfig(cfg, "newproj", localDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Save and re-read to verify persistence
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	saved := readTestConfig(t, configPath)
	if len(saved.Projects) != 2 {
		t.Fatalf("expected 2 projects in saved config, got %d", len(saved.Projects))
	}

	found := false
	for _, p := range saved.Projects {
		if p.Name == "newproj" {
			found = true
			if p.LocalPath != localDir {
				t.Errorf("saved local_path = %q, want %q", p.LocalPath, localDir)
			}
			if p.SyncPath != "newproj" {
				t.Errorf("saved sync_path = %q, want %q", p.SyncPath, "newproj")
			}
			if !p.Enabled {
				t.Error("saved project should be enabled")
			}
		}
	}
	if !found {
		t.Error("project 'newproj' not found in saved config")
	}
}
