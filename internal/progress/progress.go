// Package progress provides per-project sync progress output.
package progress

import (
	"fmt"
	"io"
	"time"

	"github.com/fatih/color"
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
		fmt.Fprintf(p.out, "%s Syncing %s...", color.CyanString("[dry-run]"), name)
	} else {
		fmt.Fprintf(p.out, "Syncing %s...", name)
	}
}

// Done completes the line with "done (<duration>)".
// If conflicts > 0, prints a conflict warning line (always, not verbose-gated).
func (p *Progress) Done(duration time.Duration, conflicts int) {
	fmt.Fprintf(p.out, " %s\n", color.GreenString("done (%s)", formatDuration(duration)))
	if conflicts > 0 {
		fmt.Fprintf(p.out, "  %s\n", color.YellowString("%d conflict(s) detected", conflicts))
		p.Hint("Run 'syncr status' to see conflict details")
	}
}

// Fail completes the line with "failed: <reason>".
// If verbose is enabled, prints local and remote paths as detail lines.
func (p *Progress) Fail(err error, localPath, remotePath string) {
	fmt.Fprintf(p.out, " %s\n", color.RedString("failed: %v", err))
	if p.verbose {
		dim := color.New(color.Faint)
		dim.Fprintf(p.out, "  local: %s\n", localPath)
		dim.Fprintf(p.out, "  sync folder: %s\n", remotePath)
	}
}

// Hint prints an indented hint line. Always prints (not verbose-gated), yellow text.
func (p *Progress) Hint(format string, args ...interface{}) {
	fmt.Fprintf(p.out, "  %s\n", color.YellowString(format, args...))
}

// Detail prints an indented detail line (only if verbose is enabled).
func (p *Progress) Detail(format string, args ...interface{}) {
	if p.verbose {
		dim := color.New(color.Faint)
		dim.Fprintf(p.out, "  "+format+"\n", args...)
	}
}

// Skip prints a standalone skip line (not part of Start/Done/Fail flow).
func (p *Progress) Skip(name, reason string) {
	if p.dryRun {
		fmt.Fprintf(p.out, "%s %s\n", color.CyanString("[dry-run]"), color.YellowString("%s: skipped (%s)", name, reason))
	} else {
		fmt.Fprintf(p.out, "%s\n", color.YellowString("%s: skipped (%s)", name, reason))
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
