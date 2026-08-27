package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/state"
)

func TestCountFiles_Excludes(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"keep.txt",
		"ignored.tmp",
		".cache/nested.txt",
	}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	count, err := countFiles(dir, []string{"*.tmp", ".cache/"})
	if err != nil {
		t.Fatalf("countFiles failed: %v", err)
	}
	if count != 1 {
		t.Errorf("countFiles = %d, want 1", count)
	}
}

func TestInitProject_UsesProvidedProjectExcludes(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "local")
	syncRoot := filepath.Join(root, "cloud")
	syncPath := filepath.Join(syncRoot, "second")
	for _, dir := range []string{local, syncPath} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(local, "second.tmp"), []byte("excluded"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		SyncRoot: syncRoot,
		Projects: []config.Project{
			{Name: "duplicate", LocalPath: root, SyncPath: "first", Enabled: true, Exclude: []string{"first.tmp"}},
			{Name: "duplicate", LocalPath: local, SyncPath: "second", Enabled: true, Exclude: []string{"second.tmp"}},
		},
	}
	cfg.SetLocalDataDir(filepath.Join(root, "state"))
	st, err := state.Load(cfg.SyncrDataDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := initProject(cfg, st, &cfg.Projects[1], false, false); err != nil {
		t.Fatalf("initProject failed: %v", err)
	}
	if !st.IsInitialized("duplicate") {
		t.Error("project was not marked initialized")
	}
	if _, err := os.Stat(filepath.Join(syncPath, "second.tmp")); !os.IsNotExist(err) {
		t.Error("file excluded by the provided project was synced")
	}
}
