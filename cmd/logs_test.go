package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogFileName(t *testing.T) {
	got := logFileName("20240115")
	want := "syncr_20240115.log"
	if got != want {
		t.Errorf("logFileName(%q) = %q, want %q", "20240115", got, want)
	}
}

func TestReadNewContent_FromStart(t *testing.T) {
	dir := t.TempDir()
	day := "20240115"
	logPath := filepath.Join(dir, logFileName(day))

	content := "line 1\nline 2\nline 3\n"
	os.WriteFile(logPath, []byte(content), 0644)

	newOffset := readNewContent(dir, day, 0)
	if newOffset != int64(len(content)) {
		t.Errorf("offset = %d, want %d", newOffset, len(content))
	}
}

func TestReadNewContent_FromMiddle(t *testing.T) {
	dir := t.TempDir()
	day := "20240115"
	logPath := filepath.Join(dir, logFileName(day))

	content := "line 1\nline 2\nline 3\n"
	os.WriteFile(logPath, []byte(content), 0644)

	// Start reading from after "line 1\n"
	startOffset := int64(len("line 1\n"))
	newOffset := readNewContent(dir, day, startOffset)
	if newOffset != int64(len(content)) {
		t.Errorf("offset = %d, want %d", newOffset, len(content))
	}
}

func TestReadNewContent_NoNewContent(t *testing.T) {
	dir := t.TempDir()
	day := "20240115"
	logPath := filepath.Join(dir, logFileName(day))

	content := "line 1\n"
	os.WriteFile(logPath, []byte(content), 0644)

	endOffset := int64(len(content))
	newOffset := readNewContent(dir, day, endOffset)
	if newOffset != endOffset {
		t.Errorf("offset = %d, want %d (no new content)", newOffset, endOffset)
	}
}

func TestReadNewContent_MissingFile(t *testing.T) {
	dir := t.TempDir()
	day := "20240115"

	// File doesn't exist - should return original offset
	offset := int64(42)
	newOffset := readNewContent(dir, day, offset)
	if newOffset != offset {
		t.Errorf("offset = %d, want %d (file missing)", newOffset, offset)
	}
}

func TestPrintTailAndGetOffset_FewLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// File with fewer than 10 lines
	lines := []string{"line 1", "line 2", "line 3"}
	os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	offset := printTailAndGetOffset(f)

	info, _ := f.Stat()
	if offset != info.Size() {
		t.Errorf("offset = %d, want %d (end of file)", offset, info.Size())
	}
}

func TestPrintTailAndGetOffset_ManyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// File with more than 10 lines
	var lines []string
	for i := 1; i <= 25; i++ {
		lines = append(lines, "line "+string(rune('0'+i/10))+string(rune('0'+i%10)))
	}
	os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	offset := printTailAndGetOffset(f)

	info, _ := f.Stat()
	if offset != info.Size() {
		t.Errorf("offset = %d, want %d (end of file)", offset, info.Size())
	}
}

func TestReadAllLines_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.log")
	os.WriteFile(path, []byte(""), 0644)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	lines, err := readAllLines(f)
	if err != nil {
		t.Fatalf("readAllLines error: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("got %d lines, want 0", len(lines))
	}
}

func TestReadAllLines_MultipleLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0644)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	lines, err := readAllLines(f)
	if err != nil {
		t.Fatalf("readAllLines error: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "alpha" || lines[1] != "beta" || lines[2] != "gamma" {
		t.Errorf("lines = %v", lines)
	}
}
