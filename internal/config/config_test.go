package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create temp directory and config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "syncr.json")

	configContent := `{
		"cloud_root": "` + tmpDir + `",
		"sync_interval_seconds": 300,
		"projects": [
			{
				"name": "TestProject",
				"local_path": "` + tmpDir + `/local",
				"cloud_subpath": "TestProject",
				"enabled": true
			}
		]
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.CloudRoot != tmpDir {
		t.Errorf("CloudRoot = %q, want %q", cfg.CloudRoot, tmpDir)
	}

	if cfg.SyncIntervalSeconds != 300 {
		t.Errorf("SyncIntervalSeconds = %d, want 300", cfg.SyncIntervalSeconds)
	}

	if len(cfg.Projects) != 1 {
		t.Fatalf("len(Projects) = %d, want 1", len(cfg.Projects))
	}

	if cfg.Projects[0].Name != "TestProject" {
		t.Errorf("Projects[0].Name = %q, want %q", cfg.Projects[0].Name, "TestProject")
	}
}

func TestLoad_DefaultInterval(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "syncr.json")

	// Config without sync_interval_seconds
	configContent := `{
		"cloud_root": "` + tmpDir + `",
		"projects": []
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SyncIntervalSeconds != 300 {
		t.Errorf("SyncIntervalSeconds = %d, want 300 (default)", cfg.SyncIntervalSeconds)
	}
}

func TestLoad_NotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/syncr.json")
	if err == nil {
		t.Error("Load() expected error for nonexistent file")
	}
}

func TestValidate(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				CloudRoot:           tmpDir,
				SyncIntervalSeconds: 300,
				Projects: []Project{
					{Name: "Test", LocalPath: tmpDir, CloudSubpath: "test", Enabled: true},
				},
			},
			wantErr: false,
		},
		{
			name: "missing cloud_root",
			cfg: Config{
				CloudRoot:           "",
				SyncIntervalSeconds: 300,
			},
			wantErr: true,
		},
		{
			name: "relative cloud_root",
			cfg: Config{
				CloudRoot:           "relative/path",
				SyncIntervalSeconds: 300,
			},
			wantErr: true,
		},
		{
			name: "nonexistent cloud_root",
			cfg: Config{
				CloudRoot:           "/nonexistent/path",
				SyncIntervalSeconds: 300,
			},
			wantErr: true,
		},
		{
			name: "interval too short",
			cfg: Config{
				CloudRoot:           tmpDir,
				SyncIntervalSeconds: 30,
			},
			wantErr: true,
		},
		{
			name: "duplicate project names",
			cfg: Config{
				CloudRoot:           tmpDir,
				SyncIntervalSeconds: 300,
				Projects: []Project{
					{Name: "Same", LocalPath: tmpDir, CloudSubpath: "a", Enabled: true},
					{Name: "Same", LocalPath: tmpDir, CloudSubpath: "b", Enabled: true},
				},
			},
			wantErr: true,
		},
		{
			name: "empty project name",
			cfg: Config{
				CloudRoot:           tmpDir,
				SyncIntervalSeconds: 300,
				Projects: []Project{
					{Name: "", LocalPath: tmpDir, CloudSubpath: "test", Enabled: true},
				},
			},
			wantErr: true,
		},
		{
			name: "relative local_path",
			cfg: Config{
				CloudRoot:           tmpDir,
				SyncIntervalSeconds: 300,
				Projects: []Project{
					{Name: "Test", LocalPath: "relative/path", CloudSubpath: "test", Enabled: true},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSyncrDataDir(t *testing.T) {
	cfg := &Config{CloudRoot: "/Users/test/OneDrive/syncr"}
	want := "/Users/test/OneDrive/syncr/_syncr"
	got := cfg.SyncrDataDir()

	if got != want {
		t.Errorf("SyncrDataDir() = %q, want %q", got, want)
	}
}

func TestGetProject(t *testing.T) {
	cfg := &Config{
		Projects: []Project{
			{Name: "ProjectA", LocalPath: "/a", Enabled: true},
			{Name: "ProjectB", LocalPath: "/b", Enabled: false},
		},
	}

	// Find existing project
	p := cfg.GetProject("ProjectA")
	if p == nil {
		t.Fatal("GetProject(ProjectA) returned nil")
	}
	if p.Name != "ProjectA" {
		t.Errorf("GetProject(ProjectA).Name = %q, want %q", p.Name, "ProjectA")
	}

	// Find nonexistent project
	p = cfg.GetProject("Nonexistent")
	if p != nil {
		t.Errorf("GetProject(Nonexistent) = %v, want nil", p)
	}
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "syncr.json")

	cfg := &Config{
		CloudRoot:           tmpDir,
		SyncIntervalSeconds: 600,
		Projects: []Project{
			{Name: "Test", LocalPath: tmpDir, CloudSubpath: "test", Enabled: true},
		},
		path: configPath,
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Reload and verify
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.SyncIntervalSeconds != 600 {
		t.Errorf("after save/load: SyncIntervalSeconds = %d, want 600", loaded.SyncIntervalSeconds)
	}

	if len(loaded.Projects) != 1 || loaded.Projects[0].Name != "Test" {
		t.Errorf("after save/load: Projects not preserved correctly")
	}
}

func TestSave_NoPath(t *testing.T) {
	cfg := &Config{
		CloudRoot:           "/tmp",
		SyncIntervalSeconds: 300,
	}
	// path is not set

	err := cfg.Save()
	if err == nil {
		t.Error("Save() expected error when path is not set")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "syncr.json")

	// Write invalid JSON
	if err := os.WriteFile(configPath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for invalid JSON")
	}
}

func TestPath(t *testing.T) {
	cfg := &Config{
		CloudRoot: "/test/path",
		path:      "/config/syncr.json",
	}

	if got := cfg.Path(); got != "/config/syncr.json" {
		t.Errorf("Path() = %q, want %q", got, "/config/syncr.json")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.CloudRoot != "" {
		t.Errorf("DefaultConfig().CloudRoot = %q, want empty", cfg.CloudRoot)
	}

	if cfg.SyncIntervalSeconds != 300 {
		t.Errorf("DefaultConfig().SyncIntervalSeconds = %d, want 300", cfg.SyncIntervalSeconds)
	}

	if cfg.Projects == nil {
		t.Error("DefaultConfig().Projects should not be nil")
	}

	if len(cfg.Projects) != 0 {
		t.Errorf("DefaultConfig().Projects length = %d, want 0", len(cfg.Projects))
	}
}

func TestValidate_EmptyCloudSubpath(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		CloudRoot:           tmpDir,
		SyncIntervalSeconds: 300,
		Projects: []Project{
			{Name: "Test", LocalPath: tmpDir, CloudSubpath: "", Enabled: true},
		},
	}

	// Empty cloud_subpath should be valid (uses project name by default)
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() should allow empty cloud_subpath, got error: %v", err)
	}
}
