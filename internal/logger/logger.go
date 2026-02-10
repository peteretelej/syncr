// Package logger provides human-friendly logging to stdout and file.
// Log files are stored in ~/.config/syncr/logs/ with daily rotation.
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fatih/color"
)

// Logger provides human-friendly logging to stdout and file.
type Logger struct {
	out        io.Writer
	file       *os.File
	verbose    bool
	logDir     string
	currentDay string
	mu         sync.Mutex
}

// New creates a new logger that writes to stdout and a log file.
// Log files are stored in {syncrDataDir}/logs/ with daily rotation.
func New(syncrDataDir string, verbose bool) (*Logger, error) {
	logDir := filepath.Join(syncrDataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	l := &Logger{
		out:     os.Stdout,
		verbose: verbose,
		logDir:  logDir,
	}

	// Open log file for today
	if err := l.rotateIfNeeded(); err != nil {
		return nil, err
	}

	// Clean up old logs
	go l.cleanOldLogs()

	return l, nil
}

// NewStdout creates a logger that only writes to stdout (no file).
func NewStdout(verbose bool) *Logger {
	return &Logger{
		out:     os.Stdout,
		verbose: verbose,
	}
}

// Info logs an informational message.
func (l *Logger) Info(format string, args ...interface{}) {
	l.log("INFO", format, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log("WARN", format, args...)
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...interface{}) {
	l.log("ERROR", format, args...)
}

// Debug logs a debug message (only if verbose mode is enabled).
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.verbose {
		l.log("DEBUG", format, args...)
	}
}

// Close closes the log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// colorizeLevel returns a colored version of the level tag for console output.
func colorizeLevel(level string) string {
	switch level {
	case "ERROR":
		return color.RedString("[ERROR]")
	case "WARN":
		return color.YellowString("[WARN]")
	case "DEBUG":
		return color.New(color.Faint).Sprint("[DEBUG]")
	default:
		return "[" + level + "]"
	}
}

func (l *Logger) log(level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)

	// Plain line for file (no ANSI codes)
	plainLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg)

	// Colored line for console
	consoleLine := fmt.Sprintf("[%s] %s %s\n", timestamp, colorizeLevel(level), msg)

	// Write colored to console
	l.out.Write([]byte(consoleLine))

	// Write plain to file if available
	if l.file != nil {
		if err := l.rotateIfNeededLocked(); err != nil {
			fmt.Fprintf(l.out, "[%s] %s log rotation failed: %v\n", timestamp, colorizeLevel("ERROR"), err)
		}
		l.file.WriteString(plainLine)
	}
}

func (l *Logger) rotateIfNeeded() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rotateIfNeededLocked()
}

func (l *Logger) rotateIfNeededLocked() error {
	if l.logDir == "" {
		return nil // No file logging
	}

	today := time.Now().Format("20060102")
	if l.currentDay == today && l.file != nil {
		return nil // Same day, no rotation needed
	}

	// Close old file
	if l.file != nil {
		l.file.Close()
	}

	// Open new file
	logPath := filepath.Join(l.logDir, fmt.Sprintf("syncr_%s.log", today))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}

	l.file = f
	l.currentDay = today
	return nil
}

func (l *Logger) cleanOldLogs() {
	if l.logDir == "" {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -7) // 7 days ago

	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Delete files older than cutoff
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(l.logDir, entry.Name()))
		}
	}
}

// SetOutput sets the output writer (for testing).
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
}
