package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/peteretelej/syncr/internal/config"
	"github.com/peteretelej/syncr/internal/discover"
	"github.com/peteretelej/syncr/internal/logger"
	"github.com/peteretelej/syncr/internal/state"
)

// Discover previews or applies configured folder discovery.
func Discover(args []string, configPath string, verbose, dryRun bool) {
	if err := runDiscover(args, configPath, verbose, dryRun, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDiscover(args []string, configPath string, verbose, dryRun bool, out, errOut io.Writer) error {
	if len(args) != 0 {
		fmt.Fprintln(errOut, "Usage: syncr [-dry-run] discover")
		return fmt.Errorf("discover accepts no positional arguments")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if cfg.Discover == nil {
		return fmt.Errorf("folder discovery not configured")
	}

	scanState, err := discover.LoadScanState(cfg)
	if err != nil {
		return err
	}
	candidates, warnings, err := discover.Scan(cfg)
	if err != nil {
		return err
	}
	plan := discover.BuildPlan(candidates, cfg, scanState)
	printDiscoveryPlan(out, plan, append(warnings, plan.Warnings...))
	if dryRun {
		fmt.Fprintln(out, "Dry run: no changes applied.")
		return nil
	}

	if err := discover.ApplyPlan(plan, cfg); err != nil {
		return err
	}
	if err := discover.SaveDiscovery(cfg, scanState); err != nil {
		return err
	}

	st, err := state.Load(cfg.SyncrDataDir())
	if err != nil {
		return fmt.Errorf("loading sync state: %w", err)
	}
	initializeDiscovered(cfg, st, plan.Adds, verbose, func(format string, args ...interface{}) {
		fmt.Fprintf(errOut, "Warning: "+format+"\n", args...)
	})
	if err := st.Save(); err != nil {
		return fmt.Errorf("saving sync state: %w", err)
	}

	fmt.Fprintf(out, "Folder discovery applied: %d added, %d disabled, %d re-enabled.\n", len(plan.Adds), len(plan.Disables), len(plan.Reenables))
	return nil
}

func printDiscoveryPlan(out io.Writer, plan discover.Plan, warnings []string) {
	fmt.Fprintln(out, "Folder discovery plan:")
	for _, addition := range plan.Adds {
		fmt.Fprintf(out, "  Add %q\n", addition.Name)
		fmt.Fprintf(out, "    Local: %s\n", addition.LocalPath)
		fmt.Fprintf(out, "    Sync:  %s\n", addition.SyncPath)
	}
	fmt.Fprintf(out, "  Keep: %d\n", len(plan.Keeps))
	for _, name := range plan.Disables {
		fmt.Fprintf(out, "  Disable %q\n", name)
	}
	for _, name := range plan.Reenables {
		fmt.Fprintf(out, "  Re-enable %q\n", name)
	}
	for _, win := range plan.ManualWins {
		fmt.Fprintf(out, "  Manual project wins for %q (%s): %s\n", win.Name, win.Reason, win.LocalPath)
	}
	for _, warning := range warnings {
		fmt.Fprintf(out, "  Warning: %s\n", warning)
	}
}

func runScheduledDiscovery(cfg *config.Config, st *state.State, verbose, dryRun bool, log *logger.Logger) (bool, error) {
	if dryRun || cfg.Discover == nil {
		return false, nil
	}
	scanState, err := discover.LoadScanState(cfg)
	if err != nil {
		return false, err
	}
	if !discover.ScanDue(*scanState, cfg) {
		return false, nil
	}

	candidates, warnings, err := discover.Scan(cfg)
	if err != nil {
		return false, err
	}
	plan := discover.BuildPlan(candidates, cfg, scanState)
	for _, warning := range append(warnings, plan.Warnings...) {
		log.Warn("Folder discovery: %s", warning)
	}
	if err := discover.ApplyPlan(plan, cfg); err != nil {
		return false, err
	}
	if err := discover.SaveDiscovery(cfg, scanState); err != nil {
		return false, err
	}
	initializeDiscovered(cfg, st, plan.Adds, verbose, func(format string, args ...interface{}) {
		log.Warn(format, args...)
	})
	log.Info("Folder discovery: %d added, %d kept, %d disabled, %d re-enabled", len(plan.Adds), len(plan.Keeps), len(plan.Disables), len(plan.Reenables))
	return true, nil
}

func initializeDiscovered(cfg *config.Config, st *state.State, additions []config.Project, verbose bool, warn func(string, ...interface{})) {
	for _, addition := range additions {
		project := cfg.GetProject(addition.Name)
		if err := initProject(cfg, st, project, verbose, false); err != nil {
			warn("Folder discovery could not initialize %q: %v", addition.Name, err)
		}
	}
}

func runSyncDiscovery(args []string, cfg *config.Config, st *state.State, verbose, dryRun bool, log *logger.Logger) (bool, error) {
	if len(args) != 0 {
		return false, nil
	}
	return runScheduledDiscovery(cfg, st, verbose, dryRun, log)
}
