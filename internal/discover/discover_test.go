package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/peteretelej/syncr/internal/config"
)

func TestScan(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		"project/_docs",
		"project/_docs/prds",
		"project/_docs/nested/_docs",
		".git/_docs",
		"node_modules/_docs",
		"excluded/_docs",
		"project/.worktrees/branch/_docs",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("_docs/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(root, filepath.Join(root, "loop")); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{Discover: &config.Discover{
		ScanRoots:    []string{root},
		FolderNames:  []string{"_docs", "prds"},
		ExcludeGlobs: []string{"excluded/**", ".worktrees"},
	}}
	candidates, _, warnings, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	got := make(map[string]bool)
	for _, candidate := range candidates {
		got[candidate.SyncPath] = true
		if !filepath.IsAbs(candidate.LocalPath) {
			t.Errorf("LocalPath = %q, want absolute", candidate.LocalPath)
		}
		if candidate.Name != strings.ReplaceAll(candidate.SyncPath, "/", "-") {
			t.Errorf("Name = %q, want derived from %q", candidate.Name, candidate.SyncPath)
		}
	}
	for _, want := range []string{"project/_docs", "project/_docs/prds"} {
		if !got[want] {
			t.Errorf("missing candidate %q; got %v", want, got)
		}
	}
	for _, unwanted := range []string{"project/_docs/nested/_docs", ".git/_docs", "node_modules/_docs", "excluded/_docs", "project/.worktrees/branch/_docs"} {
		if got[unwanted] {
			t.Errorf("unexpected candidate %q", unwanted)
		}
	}
	if runtime.GOOS != "windows" && !containsSubstring(warnings, "symlink loop skipped") {
		t.Errorf("warnings = %v, want symlink loop warning", warnings)
	}
}

func TestScanToleratesUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based unreadability is a no-op on Windows ACLs; cannot inject the condition")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.MkdirAll(filepath.Join(locked, "_docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0755) })

	cfg := &config.Config{Discover: &config.Discover{ScanRoots: []string{root}, FolderNames: []string{"_docs"}}}
	_, _, warnings, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if os.Geteuid() != 0 && !containsSubstring(warnings, "cannot read") {
		t.Errorf("warnings = %v, want unreadable-directory warning", warnings)
	}
}

func TestBuildPlanAndApply(t *testing.T) {
	root := t.TempDir()
	manualLocal := filepath.Join(root, "manual-local")
	cfg := &config.Config{
		SyncRoot:            root,
		SyncIntervalMinutes: 5,
		Projects: []config.Project{
			{Name: "same", LocalPath: filepath.Join(root, "manual-name"), SyncPath: "manual-name", Enabled: true},
			{Name: "manual-sync", LocalPath: filepath.Join(root, "manual-sync"), SyncPath: "cloud", Enabled: true},
			{Name: "manual-local", LocalPath: manualLocal, SyncPath: "manual-local", Enabled: true},
			{Name: "keep", LocalPath: filepath.Join(root, "keep"), SyncPath: "keep", Enabled: true, Discovered: true},
			{Name: "reenable", LocalPath: filepath.Join(root, "reenable"), SyncPath: "reenable", Enabled: false, Discovered: true},
			{Name: "gone", LocalPath: filepath.Join(root, "gone"), SyncPath: "gone", Enabled: true, Discovered: true},
		},
	}
	candidates := []Candidate{
		{Name: "same", LocalPath: filepath.Join(root, "candidate-name"), SyncPath: "candidate-name"},
		{Name: "candidate-sync", LocalPath: filepath.Join(root, "candidate-sync"), SyncPath: "cloud/child"},
		{Name: "candidate-local", LocalPath: filepath.Join(manualLocal, "child"), SyncPath: "candidate-local"},
		{Name: "reserved", LocalPath: filepath.Join(root, "reserved"), SyncPath: "_syncr/work"},
		{Name: "keep", LocalPath: filepath.Join(root, "keep"), SyncPath: "keep"},
		{Name: "reenable", LocalPath: filepath.Join(root, "reenable"), SyncPath: "reenable"},
		{Name: "new", LocalPath: filepath.Join(root, "new"), SyncPath: "new"},
	}
	state := &ScanState{MissedCount: map[string]int{filepath.Join(root, "gone"): VanishThreshold - 1}}
	plan := BuildPlan(candidates, ScanCoverage{Roots: []string{root}}, cfg, state)

	if len(plan.Adds) != 1 || plan.Adds[0].Name != "new" || !plan.Adds[0].Enabled || !plan.Adds[0].Discovered {
		t.Errorf("Adds = %+v, want one enabled discovered project", plan.Adds)
	}
	if strings.Join(plan.Keeps, ",") != "keep" {
		t.Errorf("Keeps = %v", plan.Keeps)
	}
	if strings.Join(plan.Reenables, ",") != "reenable" {
		t.Errorf("Reenables = %v", plan.Reenables)
	}
	if strings.Join(plan.Disables, ",") != "gone" {
		t.Errorf("Disables = %v", plan.Disables)
	}
	for _, reason := range []string{"name exists", "sync_path overlap", "inside synced folder", "reserved name"} {
		found := false
		for _, win := range plan.ManualWins {
			found = found || win.Reason == reason
		}
		if !found {
			t.Errorf("ManualWins = %+v, missing reason %q", plan.ManualWins, reason)
		}
	}
	if state.MissedCount[filepath.Join(root, "gone")] != VanishThreshold {
		t.Errorf("missed count = %d, want %d", state.MissedCount[filepath.Join(root, "gone")], VanishThreshold)
	}
	if state.MissedCount[filepath.Join(root, "keep")] != 0 || state.MissedCount[filepath.Join(root, "reenable")] != 0 {
		t.Errorf("present project counters were not reset: %v", state.MissedCount)
	}

	if err := ApplyPlan(plan, cfg); err != nil {
		t.Fatalf("ApplyPlan() error = %v", err)
	}
	if cfg.GetProject("gone").Enabled || !cfg.GetProject("reenable").Enabled {
		t.Errorf("enable flags not applied: gone=%v reenable=%v", cfg.GetProject("gone").Enabled, cfg.GetProject("reenable").Enabled)
	}
	projectCount := len(cfg.Projects)
	if err := ApplyPlan(plan, cfg); err != nil {
		t.Fatalf("second ApplyPlan() error = %v", err)
	}
	if len(cfg.Projects) != projectCount {
		t.Errorf("second apply added duplicates: %d -> %d", projectCount, len(cfg.Projects))
	}
}

func TestBuildPlanDeduplicatesCandidatesAndNeverDeletes(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		SyncRoot:            root,
		SyncIntervalMinutes: 5,
		Projects: []config.Project{
			{Name: "missing", LocalPath: filepath.Join(root, "missing"), SyncPath: "missing", Enabled: true, Discovered: true},
		},
	}
	state := &ScanState{}
	candidates := []Candidate{
		{Name: "one", LocalPath: filepath.Join(root, "one"), SyncPath: "one"},
		{Name: "one", LocalPath: filepath.Join(root, "other"), SyncPath: "other"},
		{Name: "three", LocalPath: filepath.Join(root, "three"), SyncPath: "one"},
	}
	coverage := ScanCoverage{Roots: []string{root}}
	plan := BuildPlan(candidates, coverage, cfg, state)
	if len(plan.Adds) != 1 || len(plan.Warnings) != 2 {
		t.Fatalf("plan = %+v, want one add and two warnings", plan)
	}
	for range VanishThreshold - 1 {
		plan = BuildPlan(nil, coverage, cfg, state)
	}
	if strings.Join(plan.Disables, ",") != "missing" {
		t.Fatalf("Disables = %v, want missing", plan.Disables)
	}
	if err := ApplyPlan(plan, cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Projects) != 1 {
		t.Errorf("ApplyPlan deleted projects: %+v", cfg.Projects)
	}
	plan = BuildPlan(nil, coverage, cfg, state)
	if len(plan.Disables) != 0 || state.MissedCount[filepath.Join(root, "missing")] != VanishThreshold {
		t.Errorf("disabled project counter did not stop: plan=%+v state=%+v", plan, state)
	}
}

func TestBuildPlanPreservesProjectsAfterIncompleteScans(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "area", "project", "_docs")
	tests := []struct {
		name     string
		coverage ScanCoverage
	}{
		{name: "missing root", coverage: ScanCoverage{Roots: []string{root}, Incomplete: []string{root}}},
		{name: "unreadable subtree", coverage: ScanCoverage{Roots: []string{root}, Incomplete: []string{filepath.Join(root, "area")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				SyncRoot:            t.TempDir(),
				SyncIntervalMinutes: 5,
				Projects: []config.Project{{
					Name: "project-_docs", LocalPath: projectPath, SyncPath: "project/_docs", Enabled: true, Discovered: true,
				}},
			}
			state := &ScanState{MissedCount: map[string]int{projectPath: 1}}
			for range VanishThreshold {
				plan := BuildPlan(nil, tt.coverage, cfg, state)
				if len(plan.Disables) != 0 {
					t.Fatalf("incomplete scan disabled project: %+v", plan)
				}
				if err := ApplyPlan(plan, cfg); err != nil {
					t.Fatal(err)
				}
			}
			if state.MissedCount[projectPath] != 1 || !cfg.Projects[0].Enabled {
				t.Fatalf("incomplete scans changed project state: count=%d enabled=%v", state.MissedCount[projectPath], cfg.Projects[0].Enabled)
			}
		})
	}
}

func TestScanAndPlanDoNotPersist(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "project", "_docs"), 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "syncr.json")
	configJSON := map[string]any{
		"sync_root":             root,
		"sync_interval_minutes": 5,
		"discover":              map[string]any{"scan_roots": []string{root}, "folder_names": []string{"_docs"}},
		"projects":              []any{},
	}
	data, _ := json.Marshal(configJSON)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(root, "state")
	cfg.SetLocalDataDir(stateDir)
	before, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	candidates, coverage, _, err := Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	state := &ScanState{}
	plan := BuildPlan(candidates, coverage, cfg, state)
	after, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("Scan or BuildPlan changed config mtime")
	}
	if _, err := os.Stat(filepath.Join(stateDir, scanStateFilename)); !os.IsNotExist(err) {
		t.Errorf("state file exists before save: %v", err)
	}

	if err := ApplyPlan(plan, cfg); err != nil {
		t.Fatal(err)
	}
	if err := SaveDiscovery(cfg, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Projects) != 1 || !loaded.Projects[0].Discovered {
		t.Errorf("saved projects = %+v", loaded.Projects)
	}
	loadedState, err := LoadScanState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if loadedState.LastScan.IsZero() {
		t.Error("saved LastScan is zero")
	}
}

func TestInvalidApplyAndSaveDoNotPersist(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "syncr.json")
	original := []byte(`{"sync_root":"` + filepath.ToSlash(root) + `","sync_interval_minutes":5,"projects":[]}`)
	if err := os.WriteFile(configPath, original, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetLocalDataDir(filepath.Join(root, "state"))
	invalid := Plan{Adds: []config.Project{{Name: "bad", LocalPath: "relative", SyncPath: "bad", Enabled: true, Discovered: true}}}
	if err := ApplyPlan(invalid, cfg); err == nil {
		t.Fatal("ApplyPlan() expected validation error")
	}
	if len(cfg.Projects) != 0 {
		t.Errorf("invalid mutation was not rolled back: %+v", cfg.Projects)
	}
	cfg.Projects = append(cfg.Projects, invalid.Adds[0])
	if err := SaveDiscovery(cfg, &ScanState{}); err == nil {
		t.Fatal("SaveDiscovery() expected validation error")
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Errorf("invalid config was persisted: %s", got)
	}
	if _, err := os.Stat(filepath.Join(cfg.SyncrDataDir(), scanStateFilename)); !os.IsNotExist(err) {
		t.Errorf("state file persisted for invalid config: %v", err)
	}
}

func TestScanDue(t *testing.T) {
	cfg := &config.Config{Discover: &config.Discover{ScanIntervalHours: 6}}
	if !ScanDue(ScanState{}, cfg) {
		t.Fatal("zero LastScan should be due")
	}
	if ScanDue(ScanState{LastScan: time.Now().Add(-5 * time.Hour)}, cfg) {
		t.Fatal("recent scan should not be due")
	}
	if !ScanDue(ScanState{LastScan: time.Now().Add(-7 * time.Hour)}, cfg) {
		t.Fatal("old scan should be due")
	}
	if ScanDue(ScanState{}, &config.Config{}) {
		t.Fatal("unconfigured folder discovery should not be due")
	}
}

func TestMissingDiscovered(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "existing")
	if err := os.Mkdir(existing, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Projects: []config.Project{
		{Name: "present", LocalPath: existing, Discovered: true},
		{Name: "missing", LocalPath: filepath.Join(root, "missing"), Discovered: true},
		{Name: "manual", LocalPath: filepath.Join(root, "manual")},
	}}
	missing := MissingDiscovered(cfg)
	if len(missing) != 1 || missing[0].Name != "missing" {
		t.Fatalf("MissingDiscovered() = %+v, want missing discovered project", missing)
	}
}

func containsSubstring(values []string, substring string) bool {
	for _, value := range values {
		if strings.Contains(value, substring) {
			return true
		}
	}
	return false
}
