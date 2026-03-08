package sync

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunHook_EmptyCommand(t *testing.T) {
	output, err := RunHook(context.Background(), "", t.TempDir(), HookEnv{}, 30*time.Second)
	if err != nil {
		t.Errorf("RunHook empty command: unexpected error: %v", err)
	}
	if output != "" {
		t.Errorf("RunHook empty command: expected empty output, got %q", output)
	}
}

func TestRunHook_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping Unix shell test on Windows")
	}

	output, err := RunHook(context.Background(), "echo hello", t.TempDir(), HookEnv{}, 30*time.Second)
	if err != nil {
		t.Fatalf("RunHook: unexpected error: %v", err)
	}
	if strings.TrimSpace(output) != "hello" {
		t.Errorf("RunHook output = %q, want %q", strings.TrimSpace(output), "hello")
	}
}

func TestRunHook_EnvVars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping Unix shell test on Windows")
	}

	env := HookEnv{
		ProjectName:  "testproject",
		LocalPath:    "/tmp/local",
		SyncPath:     "/tmp/sync",
		FilesChanged: 1,
		Conflicts:    3,
	}

	output, err := RunHook(context.Background(), "echo $SYNCR_PROJECT $SYNCR_FILES_CHANGED $SYNCR_CONFLICTS", t.TempDir(), env, 30*time.Second)
	if err != nil {
		t.Fatalf("RunHook: unexpected error: %v", err)
	}
	if strings.TrimSpace(output) != "testproject 1 3" {
		t.Errorf("RunHook env output = %q, want %q", strings.TrimSpace(output), "testproject 1 3")
	}
}

func TestRunHook_WorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping Unix shell test on Windows")
	}

	dir := t.TempDir()
	output, err := RunHook(context.Background(), "pwd", dir, HookEnv{}, 30*time.Second)
	if err != nil {
		t.Fatalf("RunHook: unexpected error: %v", err)
	}
	// Resolve symlinks for comparison (macOS /var -> /private/var)
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		resolved = dir
	}
	if strings.TrimSpace(output) != resolved {
		t.Errorf("RunHook pwd = %q, want %q", strings.TrimSpace(output), resolved)
	}
}

func TestRunHook_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping Unix shell test on Windows")
	}

	_, err := RunHook(context.Background(), "sleep 10", t.TempDir(), HookEnv{}, 1*time.Second)
	if err == nil {
		t.Fatal("RunHook: expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("RunHook error = %q, expected timeout message", err.Error())
	}
}

func TestRunHook_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping Unix shell test on Windows")
	}

	_, err := RunHook(context.Background(), "exit 1", t.TempDir(), HookEnv{}, 30*time.Second)
	if err == nil {
		t.Fatal("RunHook: expected error for exit 1, got nil")
	}
	if !strings.Contains(err.Error(), "hook failed") {
		t.Errorf("RunHook error = %q, expected 'hook failed'", err.Error())
	}
}

func TestRunHook_FilesChangedZero(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping Unix shell test on Windows")
	}

	env := HookEnv{FilesChanged: 0}
	output, err := RunHook(context.Background(), "echo $SYNCR_FILES_CHANGED", t.TempDir(), env, 30*time.Second)
	if err != nil {
		t.Fatalf("RunHook: unexpected error: %v", err)
	}
	if strings.TrimSpace(output) != "0" {
		t.Errorf("SYNCR_FILES_CHANGED = %q, want %q", strings.TrimSpace(output), "0")
	}
}
