// Package discover finds configured folder names and merges them into config.
package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/rclone/rclone/fs/filter"
)

// VanishThreshold is the number of consecutive missed scans before discovery
// disables a project.
const VanishThreshold = 3

const scanStateFilename = "discovery-state.json"

// Candidate is a folder found by Scan.
type Candidate struct {
	Name      string
	LocalPath string
	SyncPath  string
}

// ScanCoverage records which roots were walked and which subtrees could not be
// observed. It prevents a partial scan from treating unknown folders as gone.
type ScanCoverage struct {
	Roots      []string
	Incomplete []string
}

// ScanState tracks discovery timing and consecutive missing folders.
type ScanState struct {
	LastScan    time.Time      `json:"last_scan"`
	MissedCount map[string]int `json:"missed_count"`
}

// ManualWin describes a candidate suppressed by an existing manual project.
type ManualWin struct {
	Name      string
	LocalPath string
	Reason    string
}

// Plan describes all config changes and no-op classifications from a scan.
type Plan struct {
	Adds       []config.Project
	Excludes   map[string][]string
	Keeps      []string
	Disables   []string
	Reenables  []string
	ManualWins []ManualWin
	Warnings   []string
}

// Scan walks configured roots without reading ignore files or writing state.
func Scan(cfg *config.Config) ([]Candidate, ScanCoverage, []string, error) {
	if cfg.Discover == nil {
		return nil, ScanCoverage{}, nil, nil
	}

	fi, err := newExcludeFilter(cfg.Discover.ExcludeGlobs)
	if err != nil {
		return nil, ScanCoverage{}, nil, err
	}
	names := make(map[string]bool, len(cfg.Discover.FolderNames))
	for _, name := range cfg.Discover.FolderNames {
		names[name] = true
	}

	var candidates []Candidate
	coverage := ScanCoverage{}
	var warnings []string
	for _, configuredRoot := range cfg.Discover.ScanRoots {
		root, err := filepath.Abs(configuredRoot)
		if err != nil {
			return nil, coverage, warnings, fmt.Errorf("resolving scan root %q: %w", configuredRoot, err)
		}
		coverage.Roots = append(coverage.Roots, root)
		matched := make(map[string][]string)
		err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				coverage.Incomplete = append(coverage.Incomplete, current)
				warnings = append(warnings, fmt.Sprintf("cannot read %s: %v", current, walkErr))
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if target, err := filepath.EvalSymlinks(current); err != nil {
					warnings = append(warnings, fmt.Sprintf("cannot resolve symlink %s: %v", current, err))
				} else if isWithin(target, current) {
					warnings = append(warnings, fmt.Sprintf("symlink loop skipped: %s", current))
				}
				return nil
			}
			if !entry.IsDir() {
				return nil
			}
			if current != root && isJunkDirectory(entry.Name()) {
				coverage.Incomplete = append(coverage.Incomplete, current)
				return filepath.SkipDir
			}
			if !includedDir(fi, root, current) {
				coverage.Incomplete = append(coverage.Incomplete, current)
				return filepath.SkipDir
			}
			if current == root || !names[entry.Name()] {
				return nil
			}
			for _, ancestor := range matched[entry.Name()] {
				if ancestor != current && isWithin(ancestor, current) {
					return nil
				}
			}
			relative, err := filepath.Rel(root, current)
			if err != nil {
				return fmt.Errorf("resolving candidate path %q: %w", current, err)
			}
			syncPath := filepath.ToSlash(relative)
			candidates = append(candidates, Candidate{
				Name:      strings.ReplaceAll(syncPath, "/", "-"),
				LocalPath: current,
				SyncPath:  syncPath,
			})
			matched[entry.Name()] = append(matched[entry.Name()], current)
			return nil
		})
		if err != nil {
			return nil, coverage, warnings, fmt.Errorf("scanning %s: %w", root, err)
		}
	}
	return candidates, coverage, warnings, nil
}

// BuildPlan classifies candidates and updates the caller-owned in-memory state.
func BuildPlan(candidates []Candidate, coverage ScanCoverage, cfg *config.Config, state *ScanState) Plan {
	if state.MissedCount == nil {
		state.MissedCount = make(map[string]int)
	}
	state.LastScan = time.Now().UTC()

	plan := Plan{}
	unique := make([]Candidate, 0, len(candidates))
	seenNames := make(map[string]bool)
	seenSyncPaths := make(map[string]bool)
	seenLocalPaths := make(map[string]bool)
	for _, candidate := range candidates {
		if seenNames[candidate.Name] || seenSyncPaths[candidate.SyncPath] || seenLocalPaths[candidate.LocalPath] {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("duplicate discovery candidate skipped: %s", candidate.LocalPath))
			continue
		}
		seenNames[candidate.Name] = true
		seenSyncPaths[candidate.SyncPath] = true
		seenLocalPaths[candidate.LocalPath] = true
		unique = append(unique, candidate)
	}

	found := make(map[string]bool)
	for _, candidate := range unique {
		if manualWin, ok := classifyManualWin(candidate, cfg.Projects); ok {
			plan.ManualWins = append(plan.ManualWins, manualWin)
			continue
		}
		if isReserved(candidate.SyncPath) {
			plan.ManualWins = append(plan.ManualWins, ManualWin{Name: candidate.Name, LocalPath: candidate.LocalPath, Reason: "reserved name"})
			continue
		}

		var existing *config.Project
		for i := range cfg.Projects {
			if cfg.Projects[i].Discovered && filepath.Clean(cfg.Projects[i].LocalPath) == filepath.Clean(candidate.LocalPath) {
				existing = &cfg.Projects[i]
				break
			}
		}
		if existing != nil {
			found[existing.LocalPath] = true
			state.MissedCount[existing.LocalPath] = 0
			if existing.Enabled {
				plan.Keeps = append(plan.Keeps, existing.Name)
			} else {
				plan.Reenables = append(plan.Reenables, existing.Name)
			}
			continue
		}

		syncPath := candidate.SyncPath
		if syncPathOverlapsProjects(syncPath, cfg.Projects, plan.Adds) {
			syncPath = availableSyncPath(candidate.Name, cfg.Projects, plan.Adds)
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("nested discovery destination remapped: %s -> %s", candidate.SyncPath, syncPath))
		}
		plan.Adds = append(plan.Adds, config.Project{
			Name:       candidate.Name,
			LocalPath:  candidate.LocalPath,
			SyncPath:   syncPath,
			Enabled:    true,
			Discovered: true,
		})
		state.MissedCount[candidate.LocalPath] = 0
	}
	assignNestedExcludes(&plan, cfg.Projects, found)

	for _, project := range cfg.Projects {
		if !project.Discovered || found[project.LocalPath] || !project.Enabled {
			continue
		}
		if !coverage.observed(project.LocalPath) {
			continue
		}
		state.MissedCount[project.LocalPath]++
		if state.MissedCount[project.LocalPath] >= VanishThreshold {
			state.MissedCount[project.LocalPath] = VanishThreshold
			plan.Disables = append(plan.Disables, project.Name)
		}
	}
	return plan
}

func assignNestedExcludes(plan *Plan, projects []config.Project, found map[string]bool) {
	active := append([]config.Project(nil), plan.Adds...)
	for _, project := range projects {
		if project.Discovered && found[project.LocalPath] {
			active = append(active, project)
		}
	}
	for parentIndex := range active {
		for childIndex := range active {
			if parentIndex == childIndex || !isWithin(active[parentIndex].LocalPath, active[childIndex].LocalPath) {
				continue
			}
			relative, err := filepath.Rel(active[parentIndex].LocalPath, active[childIndex].LocalPath)
			if err != nil || relative == "." {
				continue
			}
			pattern := "/" + filepath.ToSlash(relative) + "/**"
			if slices.Contains(active[parentIndex].Exclude, pattern) {
				continue
			}
			active[parentIndex].Exclude = append(active[parentIndex].Exclude, pattern)
			if addition := projectByIdentity(plan.Adds, active[parentIndex]); addition != nil {
				addition.Exclude = append(addition.Exclude, pattern)
			} else {
				if plan.Excludes == nil {
					plan.Excludes = make(map[string][]string)
				}
				plan.Excludes[active[parentIndex].Name] = append(plan.Excludes[active[parentIndex].Name], pattern)
			}
		}
	}
}

func (coverage ScanCoverage) observed(localPath string) bool {
	for _, root := range coverage.Roots {
		if !isWithin(root, localPath) {
			continue
		}
		complete := true
		for _, incomplete := range coverage.Incomplete {
			if isWithin(root, incomplete) && isWithin(incomplete, localPath) {
				complete = false
				break
			}
		}
		if complete {
			return true
		}
	}
	return false
}

func syncPathOverlapsProjects(syncPath string, projects, additions []config.Project) bool {
	for _, project := range projects {
		if pathsOverlap(syncPath, project.SyncPath) {
			return true
		}
	}
	for _, project := range additions {
		if pathsOverlap(syncPath, project.SyncPath) {
			return true
		}
	}
	return false
}

func availableSyncPath(base string, projects, additions []config.Project) string {
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d", base, suffix)
		}
		if !syncPathOverlapsProjects(candidate, projects, additions) && !isReserved(candidate) {
			return candidate
		}
	}
}

// ApplyPlan mutates cfg without saving it and rolls back invalid results.
func ApplyPlan(plan Plan, cfg *config.Config) error {
	original := append([]config.Project(nil), cfg.Projects...)
	for _, addition := range plan.Adds {
		if projectByIdentity(cfg.Projects, addition) == nil {
			cfg.Projects = append(cfg.Projects, addition)
		}
	}
	for _, name := range plan.Disables {
		if project := cfg.GetProject(name); project != nil && project.Discovered {
			project.Enabled = false
		}
	}
	for _, name := range plan.Reenables {
		if project := cfg.GetProject(name); project != nil && project.Discovered {
			project.Enabled = true
		}
	}
	for name, patterns := range plan.Excludes {
		if project := cfg.GetProject(name); project != nil && project.Discovered {
			project.Exclude = append(project.Exclude, patterns...)
		}
	}
	if err := cfg.Validate(); err != nil {
		cfg.Projects = original
		return err
	}
	return nil
}

// LoadScanState loads local discovery state; a missing file is zero state.
func LoadScanState(cfg *config.Config) (*ScanState, error) {
	data, err := os.ReadFile(filepath.Join(cfg.SyncrDataDir(), scanStateFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return &ScanState{MissedCount: make(map[string]int)}, nil
		}
		return nil, fmt.Errorf("reading discovery state: %w", err)
	}
	var state ScanState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing discovery state: %w", err)
	}
	if state.MissedCount == nil {
		state.MissedCount = make(map[string]int)
	}
	return &state, nil
}

// SaveScanState persists local discovery state.
func SaveScanState(cfg *config.Config, state *ScanState) error {
	if err := os.MkdirAll(cfg.SyncrDataDir(), 0755); err != nil {
		return fmt.Errorf("creating discovery state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding discovery state: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.SyncrDataDir(), scanStateFilename), data, 0644); err != nil {
		return fmt.Errorf("writing discovery state: %w", err)
	}
	return nil
}

// SaveDiscovery validates and saves config before persisting scan state.
func SaveDiscovery(cfg *config.Config, state *ScanState) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	return SaveScanState(cfg, state)
}

// ScanDue reports whether configured folder discovery should run now.
func ScanDue(state ScanState, cfg *config.Config) bool {
	if cfg.Discover == nil {
		return false
	}
	interval := time.Duration(cfg.ResolvedScanIntervalHours()) * time.Hour
	return state.LastScan.IsZero() || time.Since(state.LastScan) >= interval
}

// MissingDiscovered returns discovered projects whose local folders are absent.
func MissingDiscovered(cfg *config.Config) []*config.Project {
	var missing []*config.Project
	for i := range cfg.Projects {
		project := &cfg.Projects[i]
		if !project.Discovered {
			continue
		}
		if _, err := os.Stat(project.LocalPath); os.IsNotExist(err) {
			missing = append(missing, project)
		}
	}
	return missing
}

func newExcludeFilter(patterns []string) (*filter.Filter, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	fi, err := filter.NewFilter(nil)
	if err != nil {
		return nil, err
	}
	for _, pattern := range patterns {
		if err := fi.Add(false, pattern); err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
		}
		if !strings.Contains(pattern, "/") {
			if err := fi.Add(false, "**/"+pattern+"/**"); err != nil {
				return nil, fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
			}
		}
	}
	return fi, nil
}

func includedDir(fi *filter.Filter, root, current string) bool {
	if fi == nil || current == root {
		return true
	}
	relative, err := filepath.Rel(root, current)
	if err != nil {
		return false
	}
	include, err := fi.IncludeDirectory(context.Background(), nil)(filepath.ToSlash(relative))
	return err != nil || include
}

// This list is deliberately fixed: discovery always skips only these names.
func isJunkDirectory(name string) bool {
	return name == ".git" || name == "node_modules"
}

func classifyManualWin(candidate Candidate, projects []config.Project) (ManualWin, bool) {
	for _, project := range projects {
		if project.Discovered {
			continue
		}
		if candidate.Name == project.Name {
			return ManualWin{Name: candidate.Name, LocalPath: candidate.LocalPath, Reason: "name exists"}, true
		}
		existingSyncPath := project.SyncPath
		if existingSyncPath == "" {
			existingSyncPath = project.Name
		}
		if pathsOverlap(candidate.SyncPath, existingSyncPath) {
			return ManualWin{Name: candidate.Name, LocalPath: candidate.LocalPath, Reason: "sync_path overlap"}, true
		}
		if pathsOverlap(candidate.LocalPath, project.LocalPath) {
			return ManualWin{Name: candidate.Name, LocalPath: candidate.LocalPath, Reason: "inside synced folder"}, true
		}
	}
	return ManualWin{}, false
}

func projectByIdentity(projects []config.Project, candidate config.Project) *config.Project {
	for i := range projects {
		if projects[i].Name == candidate.Name || filepath.Clean(projects[i].LocalPath) == filepath.Clean(candidate.LocalPath) || path.Clean(filepath.ToSlash(projects[i].SyncPath)) == path.Clean(candidate.SyncPath) {
			return &projects[i]
		}
	}
	return nil
}

func isReserved(syncPath string) bool {
	cleaned := path.Clean(filepath.ToSlash(syncPath))
	return cleaned == "_syncr" || strings.HasPrefix(cleaned, "_syncr/")
}

func pathsOverlap(first, second string) bool {
	first = filepath.Clean(first)
	second = filepath.Clean(second)
	return isWithin(first, second) || isWithin(second, first)
}

func isWithin(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
