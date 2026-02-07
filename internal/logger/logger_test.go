package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewStdout(t *testing.T) {
	var buf bytes.Buffer
	l := NewStdout(false)
	l.SetOutput(&buf)

	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("output should contain [INFO], got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("output should contain message, got: %s", output)
	}
}

func TestNew_CreatesLogDir(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	l, err := New(syncrDir, false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer l.Close()

	logDir := filepath.Join(syncrDir, "logs")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Error("log directory was not created")
	}
}

func TestNew_CreatesLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	l, err := New(syncrDir, false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	l.Info("test message")
	l.Close()

	// Check log file exists
	today := time.Now().Format("20060102")
	logPath := filepath.Join(syncrDir, "logs", "syncr_"+today+".log")

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	if !strings.Contains(string(content), "test message") {
		t.Errorf("log file should contain message, got: %s", content)
	}
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	l := NewStdout(false)
	l.SetOutput(&buf)

	l.Info("info message")
	l.Warn("warn message")
	l.Error("error message")

	output := buf.String()

	if !strings.Contains(output, "[INFO]") {
		t.Error("output should contain [INFO]")
	}
	if !strings.Contains(output, "[WARN]") {
		t.Error("output should contain [WARN]")
	}
	if !strings.Contains(output, "[ERROR]") {
		t.Error("output should contain [ERROR]")
	}
}

func TestDebug_VerboseMode(t *testing.T) {
	var buf bytes.Buffer
	l := NewStdout(true) // verbose = true
	l.SetOutput(&buf)

	l.Debug("debug message")

	output := buf.String()
	if !strings.Contains(output, "[DEBUG]") {
		t.Errorf("verbose mode should include DEBUG, got: %s", output)
	}
}

func TestDebug_NonVerboseMode(t *testing.T) {
	var buf bytes.Buffer
	l := NewStdout(false) // verbose = false
	l.SetOutput(&buf)

	l.Debug("debug message")

	output := buf.String()
	if strings.Contains(output, "[DEBUG]") {
		t.Errorf("non-verbose mode should not include DEBUG, got: %s", output)
	}
}

func TestLogFormat(t *testing.T) {
	var buf bytes.Buffer
	l := NewStdout(false)
	l.SetOutput(&buf)

	l.Info("formatted: %s %d", "test", 42)

	output := buf.String()
	if !strings.Contains(output, "formatted: test 42") {
		t.Errorf("format args not applied, got: %s", output)
	}
}

func TestTimestampFormat(t *testing.T) {
	var buf bytes.Buffer
	l := NewStdout(false)
	l.SetOutput(&buf)

	l.Info("test")

	output := buf.String()
	// Should start with [YYYY-MM-DD HH:MM:SS]
	if len(output) < 22 {
		t.Fatalf("output too short: %s", output)
	}

	// Check timestamp format: [2024-01-17 10:30:00]
	if output[0] != '[' {
		t.Errorf("should start with [, got: %c", output[0])
	}
	if output[11] != ' ' {
		t.Errorf("should have space at position 11, got: %c", output[11])
	}
	if output[20] != ']' {
		t.Errorf("should have ] at position 20, got: %c", output[20])
	}
}

func TestCleanOldLogs(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")
	logDir := filepath.Join(syncrDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a recent log file (today)
	recentFile := filepath.Join(logDir, "syncr_recent.log")
	if err := os.WriteFile(recentFile, []byte("recent log"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an old log file and backdate its mod time to 10 days ago
	oldFile := filepath.Join(logDir, "syncr_old.log")
	if err := os.WriteFile(oldFile, []byte("old log"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -10)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Run cleanup directly
	l := &Logger{logDir: logDir}
	l.cleanOldLogs()

	// Old file should be deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old log file should have been deleted")
	}

	// Recent file should remain
	if _, err := os.Stat(recentFile); err != nil {
		t.Error("recent log file should still exist")
	}
}

func TestCleanOldLogs_KeepsBorderlineFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}

	// File from exactly 6 days ago (within 7-day window)
	recentEnough := filepath.Join(logDir, "syncr_6days.log")
	if err := os.WriteFile(recentEnough, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	sixDaysAgo := time.Now().AddDate(0, 0, -6)
	os.Chtimes(recentEnough, sixDaysAgo, sixDaysAgo)

	// File from exactly 8 days ago (outside window)
	tooOld := filepath.Join(logDir, "syncr_8days.log")
	if err := os.WriteFile(tooOld, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	eightDaysAgo := time.Now().AddDate(0, 0, -8)
	os.Chtimes(tooOld, eightDaysAgo, eightDaysAgo)

	l := &Logger{logDir: logDir}
	l.cleanOldLogs()

	if _, err := os.Stat(recentEnough); err != nil {
		t.Error("6-day-old file should be kept")
	}
	if _, err := os.Stat(tooOld); !os.IsNotExist(err) {
		t.Error("8-day-old file should be deleted")
	}
}

func TestCleanOldLogs_NoLogDir(t *testing.T) {
	// Should not panic when logDir is empty
	l := &Logger{logDir: ""}
	l.cleanOldLogs() // no-op, just ensure no panic
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	syncrDir := filepath.Join(tmpDir, "_syncr")

	l, err := New(syncrDir, false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	l.Info("test")

	if err := l.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Closing again should be safe
	if err := l.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
}
