package progress

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestStartDone(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, false)
	p.Start("foo")
	p.Done(1*time.Second, 0)

	want := "Syncing foo... done (1.0s)\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStartFail(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, false)
	p.Start("foo")
	p.Fail(errors.New("some error"), "/local", "/remote")

	want := "Syncing foo... failed: some error\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDryRunPrefix(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, true, false)
	p.Start("foo")
	p.Done(1*time.Second, 0)

	want := "[dry-run] Syncing foo... done (1.0s)\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConflictsAlwaysShown(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, false) // verbose=false: conflicts still shown
	p.Start("foo")
	p.Done(1*time.Second, 3)

	want := "Syncing foo... done (1.0s)\n  3 conflict(s) detected\n  Run 'syncr status' to see conflict details\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestVerboseConflictsNotShownWhenZero(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, true)
	p.Start("foo")
	p.Done(1*time.Second, 0)

	want := "Syncing foo... done (1.0s)\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestVerboseFailPaths(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, true)
	p.Start("foo")
	p.Fail(errors.New("connection timeout"), "/Users/you/Projects/photos", "/Users/you/OneDrive/syncr/photos")

	want := "Syncing foo... failed: connection timeout\n" +
		"  local: /Users/you/Projects/photos\n" +
		"  sync folder: /Users/you/OneDrive/syncr/photos\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNonVerboseNoDetail(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, false)
	p.Detail("this should not appear")

	if got := buf.String(); got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

func TestVerboseDetail(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, true)
	p.Detail("extra info: %d items", 5)

	want := "  extra info: 5 items\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSkip(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, false)
	p.Skip("docs", "not initialized")

	want := "docs: skipped (not initialized)\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSkipDryRun(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, true, false)
	p.Skip("docs", "not initialized")

	want := "[dry-run] docs: skipped (not initialized)\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDryRunFail(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, true, false)
	p.Start("foo")
	p.Fail(errors.New("timeout"), "/local", "/remote")

	want := "[dry-run] Syncing foo... failed: timeout\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDryRunVerbose(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, true, true)
	p.Start("docs")
	p.Done(3200*time.Millisecond, 2)

	want := "[dry-run] Syncing docs... done (3.2s)\n  2 conflict(s) detected\n  Run 'syncr status' to see conflict details\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConflictsShownWithoutVerbose(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, false)
	p.Start("foo")
	p.Done(1*time.Second, 2)

	want := "Syncing foo... done (1.0s)\n  2 conflict(s) detected\n  Run 'syncr status' to see conflict details\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHint(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, false) // verbose=false
	p.Hint("Run 'syncr init foo --force' to resync")

	want := "  Run 'syncr init foo --force' to resync\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHintFormat(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf, false, false)
	p.Hint("Fix: %d errors in %s", 3, "myproject")

	want := "  Fix: 3 errors in myproject\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{300 * time.Millisecond, "0.3s"},
		{1 * time.Second, "1.0s"},
		{3200 * time.Millisecond, "3.2s"},
		{59900 * time.Millisecond, "59.9s"},
		{60 * time.Second, "1m0s"},
		{92 * time.Second, "1m32s"},
		{125 * time.Second, "2m5s"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
