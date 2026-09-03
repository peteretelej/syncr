package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/discover"
	"github.com/peteretelej/syncr/internal/logger"
	"github.com/peteretelej/syncr/internal/state"
)

func TestRunDiscoverDryRunPrintsFullPlanWithoutWriting(t *testing.T) {
	root := t.TempDir()
	setTestHome(t, root)
	scanRoot := filepath.Join(root, "scan")
	for _, relative := range []string{"add/_docs", "keep/_docs", "reenable/_docs", "manual/_docs"} {
		if err := os.MkdirAll(filepath.Join(scanRoot, filepath.FromSlash(relative)), 0755); err != nil {
			t.Fatal(err)
		}
	}
	syncRoot := filepath.Join(root, "sync")
	if err := os.Mkdir(syncRoot, 0755); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(scanRoot, "gone", "_docs")
	configPath := writeDiscoveryConfig(t, root, syncRoot, scanRoot, []config.Project{
		{Name: "keep", LocalPath: filepath.Join(scanRoot, "keep", "_docs"), SyncPath: "keep/_docs", Enabled: true, Discovered: true},
		{Name: "reenable", LocalPath: filepath.Join(scanRoot, "reenable", "_docs"), SyncPath: "reenable/_docs", Enabled: false, Discovered: true},
		{Name: "manual-_docs", LocalPath: filepath.Join(root, "manual-owner"), SyncPath: "manual-owner", Enabled: true},
		{Name: "gone", LocalPath: gone, SyncPath: "gone/_docs", Enabled: true, Discovered: true},
	})
	statePath := filepath.Join(root, ".config", "syncr", "discovery-state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		t.Fatal(err)
	}
	stateData, _ := json.Marshal(discover.ScanState{LastScan: time.Now().Add(-48 * time.Hour), MissedCount: map[string]int{gone: discover.VanishThreshold - 1}})
	if err := os.WriteFile(statePath, stateData, 0644); err != nil {
		t.Fatal(err)
	}
	configBefore, _ := os.ReadFile(configPath)
	stateBefore, _ := os.ReadFile(statePath)

	var out, errOut bytes.Buffer
	if err := runDiscover(nil, configPath, false, true, &out, &errOut); err != nil {
		t.Fatalf("runDiscover() error = %v", err)
	}
	for _, want := range []string{"Add \"add-_docs\"", "Keep: 1", "Disable \"gone\"", "Re-enable \"reenable\"", "Manual project wins"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
	configAfter, _ := os.ReadFile(configPath)
	stateAfter, _ := os.ReadFile(statePath)
	if !bytes.Equal(configBefore, configAfter) || !bytes.Equal(stateBefore, stateAfter) {
		t.Fatal("dry-run modified config or discovery state")
	}
}

func TestRunDiscoverAppliesPersistsAndInitializes(t *testing.T) {
	root := t.TempDir()
	setTestHome(t, root)
	scanRoot := filepath.Join(root, "scan")
	localPath := filepath.Join(scanRoot, "project", "_docs")
	if err := os.MkdirAll(localPath, 0755); err != nil {
		t.Fatal(err)
	}
	syncRoot := filepath.Join(root, "sync")
	if err := os.Mkdir(syncRoot, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := writeDiscoveryConfig(t, root, syncRoot, scanRoot, nil)

	var out, errOut bytes.Buffer
	if err := runDiscover(nil, configPath, false, false, &out, &errOut); err != nil {
		t.Fatalf("runDiscover() error = %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	project := cfg.GetProject("project-_docs")
	if project == nil || !project.Discovered || !project.Enabled {
		t.Fatalf("saved project = %+v", project)
	}
	if _, err := os.Stat(filepath.Join(root, ".config", "syncr", "discovery-state.json")); err != nil {
		t.Fatalf("discovery state not saved: %v", err)
	}
	st, err := state.Load(filepath.Join(root, ".config", "syncr"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsInitialized(project.Name) {
		t.Fatal("discovered project was not initialized")
	}
	if err := runDiscover(nil, configPath, false, false, &out, &errOut); err != nil {
		t.Fatalf("second runDiscover() error = %v", err)
	}
	cfg, _ = config.Load(configPath)
	if len(cfg.Projects) != 1 {
		t.Fatalf("idempotent rerun produced %d projects", len(cfg.Projects))
	}
}

func TestRunDiscoverNeverAddsMatchingScanRoot(t *testing.T) {
	root := t.TempDir()
	setTestHome(t, root)
	scanRoot := filepath.Join(root, "_docs")
	syncRoot := filepath.Join(root, "sync")
	if err := os.MkdirAll(scanRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(syncRoot, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := writeDiscoveryConfig(t, root, syncRoot, scanRoot, nil)

	var out, errOut bytes.Buffer
	if err := runDiscover(nil, configPath, false, false, &out, &errOut); err != nil {
		t.Fatalf("runDiscover() error = %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Projects) != 0 {
		t.Fatalf("matching scan root produced projects: %+v", cfg.Projects)
	}
	st, err := state.Load(cfg.SyncrDataDir())
	if err != nil {
		t.Fatal(err)
	}
	if st.IsInitialized(".") {
		t.Fatal("matching scan root initialized the entire sync root")
	}
}

func TestRunDiscoverAppliesDistinctNestedNames(t *testing.T) {
	root := t.TempDir()
	setTestHome(t, root)
	scanRoot := filepath.Join(root, "scan")
	syncRoot := filepath.Join(root, "sync")
	if err := os.MkdirAll(filepath.Join(scanRoot, "project", "_docs", "prds"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scanRoot, "project", "_docs", "parent.txt"), []byte("parent"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scanRoot, "project", "_docs", "prds", "child.txt"), []byte("child"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(syncRoot, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "syncr.json")
	cfg := config.Config{
		SyncRoot: syncRoot, SyncIntervalMinutes: 5,
		Discover: &config.Discover{ScanRoots: []string{scanRoot}, FolderNames: []string{"_docs", "prds"}},
		Projects: []config.Project{},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if err := runDiscover(nil, configPath, false, false, &out, &errOut); err != nil {
		t.Fatalf("runDiscover() error = %v\n%s", err, errOut.String())
	}
	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Projects) != 2 {
		t.Fatalf("nested matches produced %d projects: %+v", len(saved.Projects), saved.Projects)
	}
	if err := saved.Validate(); err != nil {
		t.Fatalf("nested matches produced invalid config: %v", err)
	}
	if paths := []string{saved.Projects[0].SyncPath, saved.Projects[1].SyncPath}; paths[0] == paths[1] || strings.HasPrefix(paths[1], paths[0]+"/") || strings.HasPrefix(paths[0], paths[1]+"/") {
		t.Fatalf("nested matches retained overlapping destinations: %v", paths)
	}
	parent := saved.GetProject("project-_docs")
	child := saved.GetProject("project-_docs-prds")
	if parent == nil || child == nil {
		t.Fatalf("nested projects not found: %+v", saved.Projects)
	}
	if !slices.Contains(parent.Exclude, "/prds/**") {
		t.Fatalf("parent excludes = %v, want child subtree excluded", parent.Exclude)
	}
	if _, err := os.Stat(filepath.Join(syncRoot, parent.SyncPath, "parent.txt")); err != nil {
		t.Fatalf("parent-owned file was not synced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(syncRoot, parent.SyncPath, "prds", "child.txt")); !os.IsNotExist(err) {
		t.Fatalf("child-owned file reached parent mirror: %v", err)
	}
	if _, err := os.Stat(filepath.Join(syncRoot, child.SyncPath, "child.txt")); err != nil {
		t.Fatalf("child-owned file was not synced by child project: %v", err)
	}
}

func TestRunDiscoverRejectsArgumentsAndMissingConfigBlock(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := runDiscover([]string{"--dry-run"}, "", false, false, &out, &errOut); err == nil || !strings.Contains(errOut.String(), "Usage:") {
		t.Fatalf("positional argument error = %v, stderr = %q", err, errOut.String())
	}

	root := t.TempDir()
	setTestHome(t, root)
	configPath := filepath.Join(root, "syncr.json")
	data, _ := json.Marshal(config.Config{SyncRoot: root, SyncIntervalMinutes: 5, Projects: []config.Project{}})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	err := runDiscover(nil, configPath, false, false, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "folder discovery not configured") {
		t.Fatalf("missing block error = %v", err)
	}
}

func TestInitializeDiscoveredWarnsAndContinues(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{SyncRoot: root, Projects: []config.Project{{Name: "missing", LocalPath: filepath.Join(root, "missing"), SyncPath: "missing", Enabled: true, Discovered: true}}}
	cfg.SetLocalDataDir(filepath.Join(root, "state"))
	st, err := state.Load(cfg.SyncrDataDir())
	if err != nil {
		t.Fatal(err)
	}
	var warning string
	initializeDiscovered(cfg, st, cfg.Projects, false, func(format string, args ...interface{}) {
		warning = strings.TrimSpace(format)
	})
	if !strings.Contains(warning, "could not initialize") {
		t.Fatalf("warning = %q", warning)
	}
}

func TestSyncDiscoverySkipsDryRunAndTargetedSync(t *testing.T) {
	root := t.TempDir()
	scanRoot := filepath.Join(root, "scan")
	if err := os.MkdirAll(filepath.Join(scanRoot, "project", "_docs"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{SyncRoot: root, SyncIntervalMinutes: 5, Discover: &config.Discover{ScanRoots: []string{scanRoot}, FolderNames: []string{"_docs"}}, Projects: []config.Project{}}
	cfg.SetLocalDataDir(filepath.Join(root, "state"))
	st, err := state.Load(cfg.SyncrDataDir())
	if err != nil {
		t.Fatal(err)
	}
	log := logger.NewStdout(false)
	if ran, err := runSyncDiscovery(nil, cfg, st, false, true, log); err != nil || ran {
		t.Fatalf("dry-run ran=%v err=%v", ran, err)
	}
	if ran, err := runSyncDiscovery([]string{"project"}, cfg, st, false, false, log); err != nil || ran {
		t.Fatalf("targeted sync ran=%v err=%v", ran, err)
	}
	if len(cfg.Projects) != 0 {
		t.Fatal("skipped folder discovery changed config")
	}
}

func TestRunScheduledDiscoveryAppliesWhenDue(t *testing.T) {
	root := t.TempDir()
	setTestHome(t, root)
	scanRoot := filepath.Join(root, "scan")
	if err := os.MkdirAll(filepath.Join(scanRoot, "project", "_docs"), 0755); err != nil {
		t.Fatal(err)
	}
	syncRoot := filepath.Join(root, "sync")
	if err := os.Mkdir(syncRoot, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := writeDiscoveryConfig(t, root, syncRoot, scanRoot, nil)
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	st, err := state.Load(cfg.SyncrDataDir())
	if err != nil {
		t.Fatal(err)
	}
	log := logger.NewStdout(false)

	ran, err := runScheduledDiscovery(cfg, st, false, false, log)
	if err != nil || !ran {
		t.Fatalf("runScheduledDiscovery() ran=%v err=%v", ran, err)
	}
	project := cfg.GetProject("project-_docs")
	if project == nil || !st.IsInitialized(project.Name) {
		t.Fatalf("discovered project was not initialized: %+v", project)
	}
	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if project := saved.GetProject("project-_docs"); project == nil || !project.Discovered {
		t.Fatalf("saved discovered project = %+v", project)
	}
	scanState, err := discover.LoadScanState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if scanState.LastScan.IsZero() {
		t.Fatal("scheduled scan did not persist LastScan")
	}
	if ran, err := runScheduledDiscovery(cfg, st, false, false, log); err != nil || ran {
		t.Fatalf("throttled rerun ran=%v err=%v", ran, err)
	}
}

func setTestHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
}

func writeDiscoveryConfig(t *testing.T, dir, syncRoot, scanRoot string, projects []config.Project) string {
	t.Helper()
	configPath := filepath.Join(dir, "syncr.json")
	cfg := config.Config{
		SyncRoot:            syncRoot,
		SyncIntervalMinutes: 5,
		Discover: &config.Discover{
			ScanRoots:    []string{scanRoot},
			FolderNames:  []string{"_docs"},
			ExcludeGlobs: []string{"node_modules"},
		},
		Projects: projects,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	return configPath
}
