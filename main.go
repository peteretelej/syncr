package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/peteretelej/syncr/cmd"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	// Global flags
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.StringVar(&configPath, "c", "", "path to config file (shorthand)")
	verbose := flag.Bool("verbose", false, "enable verbose output")
	dryRun := flag.Bool("dry-run", false, "show changes without applying")

	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "init":
		cmd.Init(args[1:], configPath, *verbose, *dryRun)
	case "sync":
		cmd.Sync(args[1:], configPath, *verbose, *dryRun)
	case "daemon":
		cmd.Daemon(configPath, *verbose)
	case "status":
		cmd.Status(configPath)
	case "config":
		cmd.ShowConfig(configPath)
	case "logs":
		cmd.Logs(args[1:], configPath)
	case "add":
		cmd.Add(args[1:], configPath, *verbose, *dryRun)
	case "enable":
		cmd.SetProjectEnabled(args[1:], configPath, true)
	case "disable":
		cmd.SetProjectEnabled(args[1:], configPath, false)
	case "version":
		cmd.Version(version, commit)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`syncr - Lightweight bidirectional folder sync

Usage:
  syncr <command> [options]

Commands:
  init [project]      Initialize project(s) (all uninitialized if no name given)
  add <name> [path]   Add a new project interactively
  sync [project]      Run sync once (all projects if no name given)
  daemon              Run continuous sync daemon
  status              Show status of all projects
  config              Show current configuration
  logs                Show today's log (use -f to follow)
  enable <project>    Enable a project for syncing
  disable <project>   Disable a project from syncing
  version             Show version information
  help                Show this help message

Options:
  -config, -c string  Path to config file (default: $SYNCR_CONFIG or ./syncr.json)
  -verbose          Enable verbose output
  -dry-run          Show what would be synced without making changes

Environment Variables:
  SYNCR_CONFIG    Path to config file (overridden by -config flag)

Examples:
  syncr init                   Initialize all uninitialized enabled projects
  syncr init MyProject        Initialize a specific project
  syncr add docs ~/Projects/docs   Add and initialize a project
  syncr sync                  Sync all enabled projects
  syncr sync MyProject        Sync specific project
  syncr daemon                Run continuous sync every 5 minutes
  syncr status                Show project status and conflicts
  syncr enable MyProject      Enable a project for syncing
  syncr disable MyProject     Disable a project from syncing`)
}
