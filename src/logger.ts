import { appendFileSync, existsSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { getPaths } from './config.js';
import type { LogLevel } from './types.js';

// ANSI color codes
const colors = {
  reset: '\x1b[0m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  white: '\x1b[37m',
};

/**
 * Get color for log level
 */
function getColor(level: LogLevel): string {
  switch (level) {
    case 'ERROR': return colors.red;
    case 'WARN': return colors.yellow;
    case 'SUCCESS': return colors.green;
    default: return colors.white;
  }
}

/**
 * Format timestamp for logging
 */
function formatTimestamp(): string {
  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, '0');
  const day = String(now.getDate()).padStart(2, '0');
  const hours = String(now.getHours()).padStart(2, '0');
  const minutes = String(now.getMinutes()).padStart(2, '0');
  const seconds = String(now.getSeconds()).padStart(2, '0');
  
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
}

/**
 * Get today's date string for log filename
 */
function getDateString(): string {
  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, '0');
  const day = String(now.getDate()).padStart(2, '0');
  
  return `${year}${month}${day}`;
}

/**
 * Get log file path for today
 */
function getLogFilePath(): string {
  const paths = getPaths();
  return join(paths.logsDir, `syncr_${getDateString()}.log`);
}

/**
 * Ensure log directory exists
 */
function ensureLogDir(): void {
  const paths = getPaths();
  if (!existsSync(paths.logsDir)) {
    mkdirSync(paths.logsDir, { recursive: true });
  }
}

/**
 * Write log entry to file
 */
function writeToFile(message: string): void {
  ensureLogDir();
  const logFile = getLogFilePath();
  appendFileSync(logFile, message + '\n');
}

/**
 * Logger class for consistent logging
 */
class Logger {
  private writeToConsole: boolean = true;
  private writeToLog: boolean = true;

  /**
   * Configure logger options
   */
  configure(options: { console?: boolean; file?: boolean }): void {
    if (options.console !== undefined) {
      this.writeToConsole = options.console;
    }
    if (options.file !== undefined) {
      this.writeToLog = options.file;
    }
  }

  /**
   * Log a message with specified level
   */
  log(message: string, level: LogLevel = 'INFO'): void {
    const timestamp = formatTimestamp();
    const logMessage = `[${timestamp}] [${level}] ${message}`;
    
    if (this.writeToConsole) {
      const color = getColor(level);
      console.log(`${color}${logMessage}${colors.reset}`);
    }
    
    if (this.writeToLog) {
      writeToFile(logMessage);
    }
  }

  /**
   * Log info message
   */
  info(message: string): void {
    this.log(message, 'INFO');
  }

  /**
   * Log success message
   */
  success(message: string): void {
    this.log(message, 'SUCCESS');
  }

  /**
   * Log warning message
   */
  warn(message: string): void {
    this.log(message, 'WARN');
  }

  /**
   * Log error message
   */
  error(message: string): void {
    this.log(message, 'ERROR');
  }

  /**
   * Log a separator line
   */
  separator(title?: string): void {
    if (title) {
      this.info(`=== ${title} ===`);
    } else {
      this.info('===');
    }
  }
}

// Export singleton instance
export const logger = new Logger();

// Export type for dependency injection
export type { Logger };
