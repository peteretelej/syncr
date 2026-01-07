/**
 * Backup source configuration
 */
export interface BackupSource {
  /** Project identifier used in backup path */
  name: string;
  /** Absolute path to the source folder */
  path: string;
  /** Whether this source is active */
  enabled: boolean;
}

/**
 * Machine-specific backup configuration
 */
export interface MachineConfig {
  /** Hostname identifier */
  machine_name: string;
  /** Array of backup sources */
  sources: BackupSource[];
}

/**
 * Result of a single backup operation
 */
export interface BackupResult {
  source: BackupSource;
  success: boolean;
  error?: string;
  duration?: number;
}

/**
 * Summary of backup run
 */
export interface BackupSummary {
  machine: string;
  startTime: Date;
  endTime: Date;
  totalSources: number;
  successCount: number;
  failCount: number;
  skippedCount: number;
  results: BackupResult[];
  dryRun: boolean;
}

/**
 * Log levels
 */
export type LogLevel = 'INFO' | 'SUCCESS' | 'WARN' | 'ERROR';

/**
 * CLI options parsed from command line
 */
export interface CliOptions {
  dryRun: boolean;
  verbose: boolean;
  listOnly: boolean;
  init: boolean;
  checkTools: boolean;
}

/**
 * Tool availability check result
 */
export interface ToolCheck {
  allGood: boolean;
  rcloneInstalled: boolean;
  rclonePath?: string;
  rcloneVersion?: string;
}

/**
 * Platform-specific paths configuration
 */
export interface PathConfig {
  backupRoot: string;
  configDir: string;
  machineConfigsDir: string;
  logsDir: string;
  backupsDir: string;
  filtersFile: string;
}
