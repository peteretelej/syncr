package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// jsonEscape returns a JSON-safe string (escaped for embedding in JSON)
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	// Remove surrounding quotes from json.Marshal output
	return string(b[1 : len(b)-1])
}

func TestLoad(t *testing.T) {
	// Create temp directory and config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "syncr.json")
	localPath := filepath.Join(tmpDir, "local")

	configContent := `{
		"sync_root": "` + jsonEscape(tmpDir) + `",
		"sync_interval_seconds": 300,
		"projects": [
			{
				"name": "TestProject",
				"local_path": "` + jsonEscape(localPath) + `",
				"sync_path": "TestProject",
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

	if cfg.SyncRoot != tmpDir {
		t.Errorf("SyncRoot = %q, want %q", cfg.SyncRoot, tmpDir)
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
		"sync_root": "` + jsonEscape(tmpDir) + `",
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
	tmpDir := t.TempDir()
	nonexistentPath := filepath.Join(tmpDir, "nonexistent", "syncr.json")
	_, err := Load(nonexistentPath)
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
				SyncRoot:            tmpDir,
				SyncIntervalSeconds: 300,
				Projects: []Project{
					{Name: "Test", LocalPath: tmpDir, SyncPath: "test", Enabled: true},
				},
			},
			wantErr: false,
		},
		{
			name: "missing sync_root",
			cfg: Config{
				SyncRoot:            "",
				SyncIntervalSeconds: 300,
			},
			wantErr: true,
		},
		{
			name: "relative sync_root",
			cfg: Config{
				SyncRoot:            "relative/path",
				SyncIntervalSeconds: 300,
			},
			wantErr: true,
		},
		{
			name: "nonexistent sync_root",
			cfg: Config{
				SyncRoot:            filepath.Join(tmpDir, "nonexistent"),
				SyncIntervalSeconds: 300,
			},
			wantErr: true,
		},
		{
			name: "interval too short",
			cfg: Config{
				SyncRoot:            tmpDir,
				SyncIntervalSeconds: 30,
			},
			wantErr: true,
		},
		{
			name: "duplicate project names",
			cfg: Config{
				SyncRoot:            tmpDir,
				SyncIntervalSeconds: 300,
				Projects: []Project{
					{Name: "Same", LocalPath: tmpDir, SyncPath: "a", Enabled: true},
					{Name: "Same", LocalPath: tmpDir, SyncPath: "b", Enabled: true},
				},
			},
			wantErr: true,
		},
		{
			name: "empty project name",
			cfg: Config{
				SyncRoot:            tmpDir,
				SyncIntervalSeconds: 300,
				Projects: []Project{
					{Name: "", LocalPath: tmpDir, SyncPath: "test", Enabled: true},
				},
			},
			wantErr: true,
		},
		{
			name: "relative local_path",
			cfg: Config{
				SyncRoot:            tmpDir,
				SyncIntervalSeconds: 300,
				Projects: []Project{
					{Name: "Test", LocalPath: "relative/path", SyncPath: "test", Enabled: true},
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
	tmpDir := t.TempDir()
	cfg := &Config{SyncRoot: tmpDir}
	want := filepath.Join(tmpDir, "_syncr")
	got := cfg.SyncrDataDir()

	if got != want {
		t.Errorf("SyncrDataDir() = %q, want %q", got, want)
	}
}

func TestGetProject(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Projects: []Project{
			{Name: "ProjectA", LocalPath: filepath.Join(tmpDir, "a"), Enabled: true},
			{Name: "ProjectB", LocalPath: filepath.Join(tmpDir, "b"), Enabled: false},
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
		SyncRoot:            tmpDir,
		SyncIntervalSeconds: 600,
		Projects: []Project{
			{Name: "Test", LocalPath: tmpDir, SyncPath: "test", Enabled: true},
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
	tmpDir := t.TempDir()
	cfg := &Config{
		SyncRoot:            tmpDir,
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
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "syncr.json")
	cfg := &Config{
		SyncRoot: tmpDir,
		path:     configPath,
	}

	if got := cfg.Path(); got != configPath {
		t.Errorf("Path() = %q, want %q", got, configPath)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.SyncRoot != "" {
		t.Errorf("DefaultConfig().SyncRoot = %q, want empty", cfg.SyncRoot)
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

func TestValidate_EmptySyncPath(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		SyncRoot:            tmpDir,
		SyncIntervalSeconds: 300,
		Projects: []Project{
			{Name: "Test", LocalPath: tmpDir, SyncPath: "", Enabled: true},
		},
	}

	// Empty sync_path should be valid (uses project name by default)
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() should allow empty sync_path, got error: %v", err)
	}
}

// TestIsOverlappingPath tests the path overlap detection with various path formats.
func TestIsOverlappingPath(t *testing.T) {
	tests := []struct {
		name     string
		path1    string
		path2    string
		expected bool
	}{
		// Unix-style paths
		{"unix parent-child overlap", "work", "work/subdir", true},
		{"unix child-parent overlap", "work/subdir", "work", true},
		{"unix deep nested overlap", "a/b", "a/b/c/d", true},
		{"unix siblings no overlap", "work/project-a", "work/project-b", false},
		{"unix different roots no overlap", "foo", "bar", false},
		{"unix same path", "work", "work", false}, // Same path handled by duplicate check
		{"unix similar prefix no overlap", "workdir", "work", false},
		{"unix similar prefix reverse", "work", "workdir", false},

		// Windows-style paths (using forward slashes as filepath.Clean normalizes)
		{"windows-style siblings", "work/project-a", "work/project-b", false},
		{"windows-style parent-child", "Documents", "Documents/Work", true},
		{"windows-style deep nested", "Users/Data", "Users/Data/Backup/2024", true},

		// Edge cases
		{"single char paths no overlap", "a", "b", false},
		{"single char parent-child", "a", "a/b", true},
		{"trailing dots normalized", "work/.", "work/subdir", true},
		{"double dots normalizes to parent", "work/foo/..", "work/bar", true}, // filepath.Clean("work/foo/..") = "work"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Normalize paths as the validation code does
			p1 := filepath.Clean(tt.path1)
			p2 := filepath.Clean(tt.path2)
			got := isOverlappingPath(p1, p2)
			if got != tt.expected {
				t.Errorf("isOverlappingPath(%q, %q) = %v, want %v", p1, p2, got, tt.expected)
			}
		})
	}
}

// TestValidate_SyncPathVariations tests sync_path validation with various path formats.
func TestValidate_SyncPathVariations(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		syncPaths []string
		wantErr   bool
	}{
		{
			name:      "simple paths valid",
			syncPaths: []string{"docs", "notes", "projects"},
			wantErr:   false,
		},
		{
			name:      "nested siblings valid",
			syncPaths: []string{"work/frontend", "work/backend", "work/shared"},
			wantErr:   false,
		},
		{
			name:      "deep nested siblings valid",
			syncPaths: []string{"2024/q1/reports", "2024/q2/reports", "2024/q3/reports"},
			wantErr:   false,
		},
		{
			name:      "mixed depth valid",
			syncPaths: []string{"simple", "nested/path", "deep/nested/path"},
			wantErr:   false,
		},
		{
			name:      "duplicate paths error",
			syncPaths: []string{"docs", "docs"},
			wantErr:   true,
		},
		{
			name:      "parent child overlap error",
			syncPaths: []string{"projects", "projects/webapp"},
			wantErr:   true,
		},
		{
			name:      "child parent overlap error",
			syncPaths: []string{"projects/webapp", "projects"},
			wantErr:   true,
		},
		{
			name:      "deep overlap error",
			syncPaths: []string{"a/b/c", "a/b/c/d/e"},
			wantErr:   true,
		},
		{
			name:      "similar prefix no overlap valid",
			syncPaths: []string{"project", "project-backup", "projects"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projects := make([]Project, len(tt.syncPaths))
			for i, sp := range tt.syncPaths {
				projects[i] = Project{
					Name:      "Project" + string(rune('A'+i)),
					LocalPath: tmpDir,
					SyncPath:  sp,
					Enabled:   true,
				}
			}

			cfg := Config{
				SyncRoot:            tmpDir,
				SyncIntervalSeconds: 300,
				Projects:            projects,
			}

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
