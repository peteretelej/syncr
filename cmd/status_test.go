package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFormatRelativeTime_JustNow(t *testing.T) {
	result := formatRelativeTime(time.Now())
	if result != "just now" {
		t.Errorf("got %q, want %q", result, "just now")
	}
}

func TestFormatRelativeTime_Minutes(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{1 * time.Minute, "1 minute ago"},
		{2 * time.Minute, "2 minutes ago"},
		{30 * time.Minute, "30 minutes ago"},
		{59 * time.Minute, "59 minutes ago"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			result := formatRelativeTime(time.Now().Add(-tt.ago))
			if result != tt.want {
				t.Errorf("got %q, want %q", result, tt.want)
			}
		})
	}
}

func TestFormatRelativeTime_Hours(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{1 * time.Hour, "1 hour ago"},
		{2 * time.Hour, "2 hours ago"},
		{23 * time.Hour, "23 hours ago"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			result := formatRelativeTime(time.Now().Add(-tt.ago))
			if result != tt.want {
				t.Errorf("got %q, want %q", result, tt.want)
			}
		})
	}
}

func TestFormatRelativeTime_Days(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{24 * time.Hour, "1 day ago"},
		{48 * time.Hour, "2 days ago"},
		{7 * 24 * time.Hour, "7 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			result := formatRelativeTime(time.Now().Add(-tt.ago))
			if result != tt.want {
				t.Errorf("got %q, want %q", result, tt.want)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1572864, "1.5 MB"},
		{2254857830, "2.1 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestCheckDaemonStatus_NoPIDFile(t *testing.T) {
	dir := t.TempDir()
	result := checkDaemonStatus(dir)
	if result != "not running" {
		t.Errorf("got %q, want %q", result, "not running")
	}
}

func TestCheckDaemonStatus_GarbagePIDFile(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "syncr.pid")
	os.WriteFile(pidFile, []byte("not-a-number"), 0644)

	result := checkDaemonStatus(dir)
	if result != "not running" {
		t.Errorf("got %q, want %q", result, "not running")
	}
}

func TestCheckDaemonStatus_StalePID(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "syncr.pid")
	// PID 99999999 almost certainly doesn't exist
	os.WriteFile(pidFile, []byte("99999999"), 0644)

	result := checkDaemonStatus(dir)
	if result != "not running" {
		t.Errorf("got %q, want %q", result, "not running")
	}
}

func TestCheckDaemonStatus_RunningProcess(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "syncr.pid")
	// Use our own PID - guaranteed to be running
	pid := os.Getpid()
	os.WriteFile(pidFile, fmt.Appendf(nil, "%d", pid), 0644)

	result := checkDaemonStatus(dir)
	want := fmt.Sprintf("running (pid %d)", pid)
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}
