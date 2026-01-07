import { existsSync, readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { hostname } from 'node:os';
import type { MachineConfig, PathConfig, BackupSource } from './types.js';

/**
 * Get the backup root directory
 * Uses current working directory - syncr backs up to wherever it runs from
 */
export function getBackupRoot(): string {
  return process.cwd();
}

/**
 * Get all path configurations
 */
export function getPaths(): PathConfig {
  const backupRoot = getBackupRoot();
  
  return {
    backupRoot,
    configDir: join(backupRoot, 'config'),
    machineConfigsDir: join(backupRoot, 'config', 'machine-configs'),
    logsDir: join(backupRoot, 'logs'),
    backupsDir: join(backupRoot, 'backups'),
    filtersFile: join(backupRoot, 'config', 'filters.txt'),
  };
}

/**
 * Get the current machine name
 */
export function getMachineName(): string {
  return hostname();
}

/**
 * Get path to machine-specific config file
 */
export function getMachineConfigPath(): string {
  const paths = getPaths();
  return join(paths.machineConfigsDir, `${getMachineName()}.json`);
}

/**
 * Validate a backup source
 */
function validateSource(source: unknown, index: number): BackupSource {
  if (typeof source !== 'object' || source === null) {
    throw new Error(`Source at index ${index} is not an object`);
  }
  
  const s = source as Record<string, unknown>;
  
  if (typeof s.name !== 'string' || s.name.trim() === '') {
    throw new Error(`Source at index ${index} missing valid 'name' field`);
  }
  
  if (typeof s.path !== 'string' || s.path.trim() === '') {
    throw new Error(`Source at index ${index} missing valid 'path' field`);
  }
  
  if (typeof s.enabled !== 'boolean') {
    throw new Error(`Source at index ${index} missing valid 'enabled' field`);
  }
  
  return {
    name: s.name.trim(),
    path: s.path.trim(),
    enabled: s.enabled,
  };
}

/**
 * Validate and parse a machine config
 */
function validateConfig(data: unknown): MachineConfig {
  if (typeof data !== 'object' || data === null) {
    throw new Error('Config must be a valid JSON object');
  }
  
  const config = data as Record<string, unknown>;
  
  if (typeof config.machine_name !== 'string') {
    throw new Error("Config missing 'machine_name' field");
  }
  
  if (!Array.isArray(config.sources)) {
    throw new Error("Config missing 'sources' array");
  }
  
  const sources = config.sources.map((s, i) => validateSource(s, i));
  
  return {
    machine_name: config.machine_name,
    sources,
  };
}

/**
 * Load and validate machine configuration
 */
export function loadConfig(configPath?: string): MachineConfig {
  const path = configPath ?? getMachineConfigPath();
  
  if (!existsSync(path)) {
    throw new Error(`Configuration file not found: ${path}\nRun with --init flag to create initial configuration`);
  }
  
  const content = readFileSync(path, 'utf-8');
  
  let data: unknown;
  try {
    data = JSON.parse(content);
  } catch (e) {
    throw new Error(`Failed to parse configuration file: ${path}\n${e instanceof Error ? e.message : String(e)}`);
  }
  
  return validateConfig(data);
}

/**
 * Create initial configuration template
 */
export function initializeConfig(): string {
  const paths = getPaths();
  const machineName = getMachineName();
  const configPath = getMachineConfigPath();
  
  // Create directories if needed
  if (!existsSync(paths.machineConfigsDir)) {
    mkdirSync(paths.machineConfigsDir, { recursive: true });
  }
  
  // Check if config already exists
  if (existsSync(configPath)) {
    return configPath;
  }
  
  // Create template config
  const template: MachineConfig = {
    machine_name: machineName,
    sources: [
      {
        name: 'ExampleProject',
        path: process.platform === 'win32' 
          ? 'C:\\Path\\To\\Project\\_docs'
          : '/path/to/project/_docs',
        enabled: false,
      },
    ],
  };
  
  writeFileSync(configPath, JSON.stringify(template, null, 2));
  
  return configPath;
}

/**
 * Check if backup root exists
 */
export function validateBackupRoot(): boolean {
  const paths = getPaths();
  return existsSync(paths.backupRoot);
}

/**
 * Get enabled sources from config
 */
export function getEnabledSources(config: MachineConfig): BackupSource[] {
  return config.sources.filter(s => s.enabled);
}
