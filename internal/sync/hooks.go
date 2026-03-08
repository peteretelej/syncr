package sync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

// HookEnv provides context to hook commands via environment variables.
type HookEnv struct {
	ProjectName  string
	LocalPath    string
	SyncPath     string
	FilesChanged int
	Conflicts    int
}

// RunHook executes a shell command with the given environment and timeout.
// Returns combined stdout/stderr output and any error.
// An empty command is a no-op that returns ("", nil).
func RunHook(ctx context.Context, command string, workDir string, env HookEnv, timeout time.Duration) (string, error) {
	if command == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", command)
	} else {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		cmd = exec.CommandContext(ctx, shell, "-c", command)
	}

	cmd.Dir = workDir

	// Inherit parent environment and add syncr-specific vars
	cmd.Env = append(os.Environ(),
		"SYNCR_PROJECT="+env.ProjectName,
		"SYNCR_LOCAL_PATH="+env.LocalPath,
		"SYNCR_SYNC_PATH="+env.SyncPath,
		"SYNCR_HAS_CHANGES="+boolStr(env.FilesChanged),
		"SYNCR_CONFLICTS="+strconv.Itoa(env.Conflicts),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("hook timed out after %v: %w", timeout, err)
		}
		return string(output), fmt.Errorf("hook failed: %w", err)
	}

	return string(output), nil
}

// boolStr returns "1" if v is non-zero, "0" otherwise.
func boolStr(v int) string {
	if v != 0 {
		return "1"
	}
	return "0"
}
