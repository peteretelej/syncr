import { existsSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { 
  loadConfig, 
  initializeConfig, 
  getPaths, 
  getMachineName, 
  getMachineConfigPath,
  getEnabledSources,
  validateBackupRoot,
} from './config.js';
import { logger } from './logger.js';
import { checkRclone, executeRclone } from './rclone.js';
import type { 
  BackupSource, 
  BackupResult, 
  BackupSummary, 
  CliOptions,
  MachineConfig,
} from './types.js';

/**
 * Run tool check and display results
 */
export function runToolCheck(): void {
  logger.separator('Tool Check');
  
  const toolCheck = checkRclone();
  
  if (toolCheck.rcloneInstalled) {
    logger.success(`rclone: INSTALLED`);
    if (toolCheck.rclonePath) {
      logger.info(`  Path: ${toolCheck.rclonePath}`);
    }
    if (toolCheck.rcloneVersion) {
      logger.info(`  Version: ${toolCheck.rcloneVersion}`);
    }
  } else {
    logger.error('rclone: MISSING');
    logger.info('  Install from: https://rclone.org/downloads/');
  }
}

/**
 * Run initialization and create config
 */
export function runInit(): void {
  const machineName = getMachineName();
  logger.info(`Initializing backup configuration for machine: ${machineName}`);
  
  const configPath = initializeConfig();
  
  if (existsSync(configPath)) {
    logger.success(`Configuration created: ${configPath}`);
    logger.info('Edit the file and add your source directories, then set enabled = true');
  }
}

/**
 * List configured sources
 */
export function runListSources(): void {
  const configPath = getMachineConfigPath();
  
  if (!existsSync(configPath)) {
    logger.error(`Configuration file not found: ${configPath}`);
    logger.info('Run with --init flag to create initial configuration');
    process.exit(1);
  }
  
  const config = loadConfig(configPath);
  
  logger.separator('Configured Sources');
  
  for (const source of config.sources) {
    const status = source.enabled ? 'ENABLED' : 'DISABLED';
    const exists = existsSync(source.path) ? 'EXISTS' : 'NOT FOUND';
    logger.info(`${source.name}: ${status} - ${exists} - ${source.path}`);
  }
}

/**
 * Backup a single source
 */
async function backupSource(
  source: BackupSource,
  options: { dryRun: boolean; verbose: boolean }
): Promise<BackupResult> {
  const startTime = Date.now();
  const paths = getPaths();
  const machineName = getMachineName();
  
  logger.info(`Processing: ${source.name}`);
  
  // Validate source path exists
  if (!existsSync(source.path)) {
    logger.warn(`  Source not found: ${source.path}`);
    return {
      source,
      success: false,
      error: 'Source path not found',
    };
  }
  
  // Create destination path
  const destPath = join(paths.backupsDir, machineName, source.name);
  if (!existsSync(destPath)) {
    mkdirSync(destPath, { recursive: true });
  }
  
  logger.info(`  From: ${source.path}`);
  logger.info(`  To:   ${destPath}`);
  
  try {
    await executeRclone(source.path, destPath, {
      dryRun: options.dryRun,
      verbose: options.verbose,
    });
    
    const duration = Date.now() - startTime;
    logger.success(`  Status: Success (${duration}ms)`);
    
    return {
      source,
      success: true,
      duration,
    };
  } catch (err) {
    const errorMessage = err instanceof Error ? err.message : String(err);
    logger.error(`  Status: Failed - ${errorMessage}`);
    
    return {
      source,
      success: false,
      error: errorMessage,
    };
  }
}

/**
 * Run the main backup process
 */
export async function runBackup(options: CliOptions): Promise<void> {
  const startTime = new Date();
  const machineName = getMachineName();
  const paths = getPaths();
  
  // Handle special modes
  if (options.checkTools) {
    runToolCheck();
    return;
  }
  
  if (options.init) {
    runInit();
    return;
  }
  
  if (options.listOnly) {
    runListSources();
    return;
  }
  
  // Start backup
  logger.separator('Starting Backup');
  logger.info(`Machine: ${machineName}`);
  
  if (options.dryRun) {
    logger.warn('Mode: DRY RUN');
  }
  
  // Validate backup root exists
  if (!validateBackupRoot()) {
    logger.error(`Backup root directory not found: ${paths.backupRoot}`);
    process.exit(1);
  }
  
  // Check rclone is installed
  const toolCheck = checkRclone();
  if (!toolCheck.allGood) {
    logger.error('rclone is not installed');
    logger.info('  Install from: https://rclone.org/downloads/');
    process.exit(1);
  }
  
  // Load configuration
  const configPath = getMachineConfigPath();
  
  if (!existsSync(configPath)) {
    logger.error(`No configuration found for this machine`);
    logger.error(`Machine-specific config expected at: ${configPath}`);
    logger.info('Run with --init flag to create initial configuration');
    process.exit(1);
  }
  
  let config: MachineConfig;
  try {
    config = loadConfig(configPath);
    logger.info(`Loaded configuration from: ${configPath}`);
  } catch (err) {
    logger.error(err instanceof Error ? err.message : String(err));
    process.exit(1);
  }
  
  // Get enabled sources
  const enabledSources = getEnabledSources(config);
  
  if (enabledSources.length === 0) {
    logger.warn('No enabled sources found in configuration');
    return;
  }
  
  logger.info(`Processing ${enabledSources.length} source(s)`);
  
  // Process each source
  const results: BackupResult[] = [];
  
  for (const source of enabledSources) {
    const result = await backupSource(source, {
      dryRun: options.dryRun,
      verbose: options.verbose,
    });
    results.push(result);
  }
  
  // Calculate summary
  const endTime = new Date();
  const successCount = results.filter(r => r.success).length;
  const failCount = results.filter(r => !r.success).length;
  
  // Display summary
  logger.separator('Summary');
  logger.success(`Success: ${successCount}`);
  
  if (failCount > 0) {
    logger.warn(`Failed: ${failCount}`);
  }
  
  if (options.dryRun) {
    logger.warn('DRY RUN - no files copied');
  }
  
  // Return summary for programmatic use
  const summary: BackupSummary = {
    machine: machineName,
    startTime,
    endTime,
    totalSources: enabledSources.length,
    successCount,
    failCount,
    skippedCount: config.sources.length - enabledSources.length,
    results,
    dryRun: options.dryRun,
  };
  
  // Exit with error if any failures
  if (failCount > 0 && !options.dryRun) {
    process.exit(1);
  }
}
