package config

import (
	"encoding/json"
	"fmt"
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
		"sync_interval_minutes": 5,
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

	if cfg.SyncIntervalMinutes != 5 {
		t.Errorf("SyncIntervalMinutes = %d, want 5", cfg.SyncIntervalMinutes)
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

	// Config without sync_interval_minutes
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

	if cfg.SyncIntervalMinutes != 5 {
		t.Errorf("SyncIntervalMinutes = %d, want 5 (default)", cfg.SyncIntervalMinutes)
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
				SyncIntervalMinutes: 300,
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
				SyncIntervalMinutes: 300,
			},
			wantErr: true,
		},
		{
			name: "relative sync_root",
			cfg: Config{
				SyncRoot:            "relative/path",
				SyncIntervalMinutes: 300,
			},
			wantErr: true,
		},
		{
			name: "nonexistent sync_root",
			cfg: Config{
				SyncRoot:            filepath.Join(tmpDir, "nonexistent"),
				SyncIntervalMinutes: 300,
			},
			wantErr: true,
		},
		{
			name: "interval too short",
			cfg: Config{
				SyncRoot:            tmpDir,
				SyncIntervalMinutes: 0,
			},
			wantErr: true,
		},
		{
			name: "duplicate project names",
			cfg: Config{
				SyncRoot:            tmpDir,
				SyncIntervalMinutes: 300,
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
				SyncIntervalMinutes: 300,
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
				SyncIntervalMinutes: 300,
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

	// With localDataDir set, returns the local path
	cfg := &Config{SyncRoot: tmpDir}
	localDir := filepath.Join(tmpDir, "local")
	cfg.SetLocalDataDir(localDir)
	got := cfg.SyncrDataDir()
	if got != localDir {
		t.Errorf("SyncrDataDir() = %q, want %q", got, localDir)
	}
}

func TestSyncrDataDir_PanicsWithoutLocalDataDir(t *testing.T) {
	cfg := &Config{SyncRoot: t.TempDir()}

	defer func() {
		if r := recover(); r == nil {
			t.Error("SyncrDataDir() should panic when localDataDir is not set")
		}
	}()

	cfg.SyncrDataDir()
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
		SyncIntervalMinutes: 10,
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

	if loaded.SyncIntervalMinutes != 10 {
		t.Errorf("after save/load: SyncIntervalMinutes = %d, want 10", loaded.SyncIntervalMinutes)
	}

	if len(loaded.Projects) != 1 || loaded.Projects[0].Name != "Test" {
		t.Errorf("after save/load: Projects not preserved correctly")
	}
}

func TestSave_NoPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		SyncRoot:            tmpDir,
		SyncIntervalMinutes: 300,
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

	if cfg.SyncIntervalMinutes != 5 {
		t.Errorf("DefaultConfig().SyncIntervalMinutes = %d, want 5", cfg.SyncIntervalMinutes)
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
		SyncIntervalMinutes: 300,
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

func Test_dataDir(t *testing.T) {
	dir, err := dataDir()
	if err != nil {
		t.Fatalf("dataDir() error = %v", err)
	}
	if dir == "" {
		t.Fatal("dataDir() returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("dataDir() = %q, want absolute path", dir)
	}
	want := filepath.Join(".config", "syncr")
	if len(dir) < len(want) || dir[len(dir)-len(want):] != want {
		t.Errorf("dataDir() = %q, want path ending in %q", dir, want)
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

func TestValidationResult_OK(t *testing.T) {
	tests := []struct {
		name string
		vr   ValidationResult
		want bool
	}{
		{"no issues", ValidationResult{}, true},
		{"warnings only", ValidationResult{Warnings: []string{"warn"}}, true},
		{"errors only", ValidationResult{Errors: []string{"err"}}, false},
		{"both", ValidationResult{Errors: []string{"err"}, Warnings: []string{"warn"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vr.OK(); got != tt.want {
				t.Errorf("OK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationResult_HasIssues(t *testing.T) {
	tests := []struct {
		name string
		vr   ValidationResult
		want bool
	}{
		{"no issues", ValidationResult{}, false},
		{"warnings only", ValidationResult{Warnings: []string{"warn"}}, true},
		{"errors only", ValidationResult{Errors: []string{"err"}}, true},
		{"both", ValidationResult{Errors: []string{"err"}, Warnings: []string{"warn"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vr.HasIssues(); got != tt.want {
				t.Errorf("HasIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateFull_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "local")
	cloudDir := filepath.Join(tmpDir, "myproject")
	os.MkdirAll(localDir, 0755)
	os.MkdirAll(cloudDir, 0755)

	cfg := Config{
		SyncRoot:            tmpDir,
		SyncIntervalMinutes: 300,
		Projects: []Project{
			{Name: "myproject", LocalPath: localDir, SyncPath: "myproject", Enabled: true},
		},
	}

	result := cfg.ValidateFull()
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
	if len(result.Warnings) > 0 {
		t.Errorf("expected no warnings, got %v", result.Warnings)
	}
}

func TestValidateFull_MissingSyncRoot(t *testing.T) {
	cfg := Config{
		SyncRoot:            "",
		SyncIntervalMinutes: 300,
	}
	result := cfg.ValidateFull()
	if len(result.Errors) == 0 {
		t.Fatal("expected error for missing sync_root")
	}
	if result.Errors[0] != "sync_root is required" {
		t.Errorf("unexpected error: %s", result.Errors[0])
	}
}

func TestValidateFull_RelativeSyncRoot(t *testing.T) {
	cfg := Config{
		SyncRoot:            "relative/path",
		SyncIntervalMinutes: 300,
	}
	result := cfg.ValidateFull()
	if len(result.Errors) == 0 {
		t.Fatal("expected error for relative sync_root")
	}
	if result.Errors[0] != "sync_root must be an absolute path" {
		t.Errorf("unexpected error: %s", result.Errors[0])
	}
}

func TestValidateFull_NonexistentSyncRoot(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		SyncRoot:            filepath.Join(tmpDir, "nonexistent"),
		SyncIntervalMinutes: 300,
	}
	result := cfg.ValidateFull()
	// Should be a warning, not an error
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors for nonexistent sync_root, got %v", result.Errors)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for nonexistent sync_root")
	}
	found := false
	for _, w := range result.Warnings {
		if w == fmt.Sprintf("sync_root directory does not exist: %s", filepath.Join(tmpDir, "nonexistent")) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected sync_root warning, got %v", result.Warnings)
	}
}

func TestValidateFull_IntervalBelowMinimum(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		SyncRoot:            tmpDir,
		SyncIntervalMinutes: 0,
	}
	result := cfg.ValidateFull()
	if len(result.Errors) == 0 {
		t.Fatal("expected error for interval below minimum")
	}
	found := false
	for _, e := range result.Errors {
		if e == "sync_interval_minutes must be at least 1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected interval error, got %v", result.Errors)
	}
}

func TestValidateFull_DuplicateProjectNames(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		SyncRoot:            tmpDir,
		SyncIntervalMinutes: 300,
		Projects: []Project{
			{Name: "dup", LocalPath: tmpDir, SyncPath: "a", Enabled: true},
			{Name: "dup", LocalPath: tmpDir, SyncPath: "b", Enabled: true},
		},
	}
	result := cfg.ValidateFull()
	found := false
	for _, e := range result.Errors {
		if e == "duplicate project name: dup" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate name error, got %v", result.Errors)
	}
}

func TestValidateFull_OverlappingSyncPaths(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		SyncRoot:            tmpDir,
		SyncIntervalMinutes: 300,
		Projects: []Project{
			{Name: "parent", LocalPath: tmpDir, SyncPath: "work", Enabled: true},
			{Name: "child", LocalPath: tmpDir, SyncPath: "work/sub", Enabled: true},
		},
	}
	result := cfg.ValidateFull()
	found := false
	for _, e := range result.Errors {
		if e == `overlapping sync_path: "work/sub" and "work" would conflict` {
			found = true
		}
	}
	if !found {
		t.Errorf("expected overlapping sync_path error, got %v", result.Errors)
	}
}

func TestValidateFull_NonexistentLocalPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		SyncRoot:            tmpDir,
		SyncIntervalMinutes: 300,
		Projects: []Project{
			{Name: "proj", LocalPath: filepath.Join(tmpDir, "nope"), SyncPath: "proj", Enabled: true},
		},
	}
	result := cfg.ValidateFull()
	found := false
	for _, w := range result.Warnings {
		if w == fmt.Sprintf("project proj: local_path does not exist: %s", filepath.Join(tmpDir, "nope")) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected local_path warning, got warnings: %v", result.Warnings)
	}
}

func TestValidateFull_NonexistentCloudPath(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "local")
	os.MkdirAll(localDir, 0755)

	cfg := Config{
		SyncRoot:            tmpDir,
		SyncIntervalMinutes: 300,
		Projects: []Project{
			{Name: "proj", LocalPath: localDir, SyncPath: "proj", Enabled: true},
		},
	}
	result := cfg.ValidateFull()
	found := false
	syncFolderPath := filepath.Join(tmpDir, "proj")
	for _, w := range result.Warnings {
		if w == fmt.Sprintf("project proj: sync folder does not exist: %s", syncFolderPath) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected sync folder warning, got warnings: %v", result.Warnings)
	}
}

func TestValidateFull_NoEnabledProjects(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		SyncRoot:            tmpDir,
		SyncIntervalMinutes: 300,
		Projects: []Project{
			{Name: "proj", LocalPath: tmpDir, SyncPath: "proj", Enabled: false},
		},
	}
	result := cfg.ValidateFull()
	found := false
	for _, w := range result.Warnings {
		if w == "no enabled projects" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'no enabled projects' warning, got warnings: %v", result.Warnings)
	}
}

func TestValidateFull_MultipleIssues(t *testing.T) {
	cfg := Config{
		SyncRoot:            "",
		SyncIntervalMinutes: 0,
		Projects: []Project{
			{Name: "", LocalPath: "", Enabled: false},
		},
	}
	result := cfg.ValidateFull()
	if len(result.Errors) < 3 {
		t.Errorf("expected at least 3 errors (sync_root, interval, project name), got %d: %v", len(result.Errors), result.Errors)
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
				SyncIntervalMinutes: 300,
				Projects:            projects,
			}

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
