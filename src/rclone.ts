import { spawn, spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { getPaths } from './config.js';
import { logger } from './logger.js';
import type { ToolCheck } from './types.js';

/**
 * Find rclone executable
 */
function findRclone(): string | null {
  // Try to find rclone in PATH
  const command = process.platform === 'win32' ? 'where' : 'which';
  const result = spawnSync(command, ['rclone'], {
    encoding: 'utf-8',
    windowsHide: true,
  });
  
  if (result.status === 0 && result.stdout) {
    return result.stdout.trim().split('\n')[0];
  }
  
  return null;
}

/**
 * Get rclone version
 */
function getRcloneVersion(): string | null {
  const result = spawnSync('rclone', ['version'], {
    encoding: 'utf-8',
    windowsHide: true,
  });
  
  if (result.status === 0 && result.stdout) {
    // Extract version from first line: "rclone v1.x.x"
    const match = result.stdout.match(/rclone\s+(v[\d.]+)/);
    return match ? match[1] : result.stdout.split('\n')[0];
  }
  
  return null;
}

/**
 * Check if rclone is installed and available
 */
export function checkRclone(): ToolCheck {
  const rclonePath = findRclone();
  const rcloneInstalled = rclonePath !== null;
  const rcloneVersion = rcloneInstalled ? getRcloneVersion() : undefined;
  
  return {
    allGood: rcloneInstalled,
    rcloneInstalled,
    rclonePath: rclonePath ?? undefined,
    rcloneVersion: rcloneVersion ?? undefined,
  };
}

/**
 * Build rclone command arguments
 */
export function buildRcloneArgs(
  sourcePath: string,
  destPath: string,
  options: { dryRun?: boolean; verbose?: boolean } = {}
): string[] {
  const paths = getPaths();
  const args: string[] = [
    'sync',
    sourcePath,
    destPath,
    '--checksum',
    '--delete-during',
  ];
  
  if (options.dryRun) {
    args.push('--dry-run');
  }
  
  if (options.verbose) {
    args.push('--verbose');
    args.push('--progress');
  }
  
  // Add filter file if it exists
  if (existsSync(paths.filtersFile)) {
    args.push('--filter-from', paths.filtersFile);
  }
  
  return args;
}

/**
 * Execute rclone sync command
 * Returns a promise that resolves on success, rejects on failure
 */
export function executeRclone(
  sourcePath: string,
  destPath: string,
  options: { dryRun?: boolean; verbose?: boolean } = {}
): Promise<void> {
  return new Promise((resolve, reject) => {
    const args = buildRcloneArgs(sourcePath, destPath, options);
    
    if (options.verbose) {
      logger.info(`  Command: rclone ${args.join(' ')}`);
    }
    
    const proc = spawn('rclone', args, {
      stdio: options.verbose ? 'inherit' : 'pipe',
      windowsHide: true,
    });
    
    let stderr = '';
    
    if (!options.verbose && proc.stderr) {
      proc.stderr.on('data', (data: Buffer) => {
        stderr += data.toString();
      });
    }
    
    proc.on('close', (code: number | null) => {
      if (code === 0) {
        resolve();
      } else {
        reject(new Error(stderr || `rclone exited with code ${code}`));
      }
    });
    
    proc.on('error', (err: Error) => {
      reject(new Error(`Failed to spawn rclone: ${err.message}`));
    });
  });
}

/**
 * Get rclone command string for display
 */
export function getRcloneCommandString(
  sourcePath: string,
  destPath: string,
  options: { dryRun?: boolean; verbose?: boolean } = {}
): string {
  const args = buildRcloneArgs(sourcePath, destPath, options);
  return `rclone ${args.join(' ')}`;
}
