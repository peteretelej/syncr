#!/usr/bin/env bun
import { Command } from 'commander';
import { runBackup } from './backup.js';
import type { CliOptions } from './types.js';

const program = new Command();

program
  .name('docs-backup')
  .description('Cross-platform backup tool using rclone to sync local folders to OneDrive')
  .version('1.0.0');

program
  .option('-d, --dry-run', 'Run without making changes', false)
  .option('-v, --verbose', 'Show detailed output', false)
  .option('-l, --list-only', 'List configured sources and exit', false)
  .option('-i, --init', 'Initialize configuration for this machine', false)
  .option('-c, --check-tools', 'Check if required tools are installed', false);

program.parse();

const opts = program.opts();

const options: CliOptions = {
  dryRun: opts.dryRun ?? false,
  verbose: opts.verbose ?? false,
  listOnly: opts.listOnly ?? false,
  init: opts.init ?? false,
  checkTools: opts.checkTools ?? false,
};

// Run backup with parsed options
runBackup(options).catch((err: Error) => {
  console.error('Fatal error:', err.message);
  process.exit(1);
});
