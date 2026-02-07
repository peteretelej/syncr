// Package progress provides per-project sync progress output.
// Plain text only, no ANSI codes or cursor movement.
package progress

import (
	"fmt"
	"io"
	"time"
)

// Progress writes per-project sync progress to an output writer.
type Progress struct {
	out     io.Writer
	dryRun  bool
	verbose bool
}

// New creates a Progress writer.
// Set out to os.Stdout for normal use, or a bytes.Buffer for testing.
func New(out io.Writer, dryRun, verbose bool) *Progress {
	return &Progress{
		out:     out,
		dryRun:  dryRun,
		verbose: verbose,
	}
}

// Start prints the "Syncing <name>..." prefix.
// Call exactly once per project, before Done or Fail.
func (p *Progress) Start(name string) {
	if p.dryRun {
		fmt.Fprintf(p.out, "[dry-run] Syncing %s...", name)
	} else {
		fmt.Fprintf(p.out, "Syncing %s...", name)
	}
}

// Done completes the line with "done (<duration>)".
// If conflicts > 0 and verbose is enabled, prints a detail line.
func (p *Progress) Done(duration time.Duration, conflicts int) {
	fmt.Fprintf(p.out, " done (%s)\n", formatDuration(duration))
	if p.verbose && conflicts > 0 {
		fmt.Fprintf(p.out, "  %d conflict(s) detected\n", conflicts)
	}
}

// Fail completes the line with "failed: <reason>".
// If verbose is enabled, prints local and remote paths as detail lines.
func (p *Progress) Fail(err error, localPath, remotePath string) {
	fmt.Fprintf(p.out, " failed: %v\n", err)
	if p.verbose {
		fmt.Fprintf(p.out, "  local: %s\n", localPath)
		fmt.Fprintf(p.out, "  remote: %s\n", remotePath)
	}
}

// Detail prints an indented detail line (only if verbose is enabled).
func (p *Progress) Detail(format string, args ...interface{}) {
	if p.verbose {
		fmt.Fprintf(p.out, "  "+format+"\n", args...)
	}
}

// Skip prints a standalone skip line (not part of Start/Done/Fail flow).
func (p *Progress) Skip(name, reason string) {
	if p.dryRun {
		fmt.Fprintf(p.out, "[dry-run] %s: skipped (%s)\n", name, reason)
	} else {
		fmt.Fprintf(p.out, "%s: skipped (%s)\n", name, reason)
	}
}

// formatDuration produces human-friendly duration strings.
// Under 1s: "0.3s", 1-60s: "3.2s", over 60s: "1m32s".
func formatDuration(d time.Duration) string {
	secs := d.Seconds()
	if secs < 60 {
		return fmt.Sprintf("%.1fs", secs)
	}
	m := int(secs) / 60
	s := int(secs) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}
