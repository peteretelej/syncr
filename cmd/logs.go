package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/peteretelej/syncr/internal/config"
)

// Logs displays recent log output or follows live log output.
func Logs(args []string, configPath string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	var follow bool
	fs.BoolVar(&follow, "f", false, "follow log output")
	fs.BoolVar(&follow, "follow", false, "follow log output")
	fs.Parse(args)

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	logDir := filepath.Join(cfg.SyncrDataDir(), "logs")

	if follow {
		followLogs(logDir)
	} else {
		showLogs(logDir)
	}
}

// logFileName returns the log file name for the given date string (YYYYMMDD).
func logFileName(day string) string {
	return fmt.Sprintf("syncr_%s.log", day)
}

// showLogs prints the last 50 lines of today's log file.
func showLogs(logDir string) {
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		fmt.Println("No logs found.")
		return
	}

	today := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, logFileName(today))

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No logs for today.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error reading log file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	lines, err := readAllLines(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading log file: %v\n", err)
		os.Exit(1)
	}

	if len(lines) == 0 {
		return
	}

	// Print last 50 lines
	start := 0
	if len(lines) > 50 {
		start = len(lines) - 50
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}
}

// followLogs tails the log file, printing new lines as they appear.
func followLogs(logDir string) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	currentDay := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, logFileName(currentDay))

	var offset int64

	// Try to open the current log file
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Waiting for log file...")
		} else {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Print last 10 lines for context, then seek to end
		offset = printTailAndGetOffset(f)
		f.Close()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			return
		case <-ticker.C:
			// Check for daily rotation
			today := time.Now().Format("20060102")
			if today != currentDay {
				// Read remaining content from old file
				offset = readNewContent(logDir, currentDay, offset)
				// Switch to new day
				currentDay = today
				offset = 0
			}

			logPath = filepath.Join(logDir, logFileName(currentDay))

			info, err := os.Stat(logPath)
			if err != nil {
				continue // File doesn't exist yet, keep waiting
			}

			if info.Size() <= offset {
				continue // No new content
			}

			// Read new content
			offset = readNewContent(logDir, currentDay, offset)
		}
	}
}

// readNewContent reads and prints any new content from the log file starting at offset.
// Returns the new offset after reading.
func readNewContent(logDir, day string, offset int64) int64 {
	logPath := filepath.Join(logDir, logFileName(day))

	f, err := os.Open(logPath)
	if err != nil {
		return offset
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	newOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return offset
	}
	return newOffset
}

// printTailAndGetOffset prints the last 10 lines of the file and returns
// the offset at end of file for subsequent follow reads.
func printTailAndGetOffset(f *os.File) int64 {
	lines, err := readAllLines(f)
	if err != nil {
		return 0
	}

	// Print last 10 lines for context
	start := 0
	if len(lines) > 10 {
		start = len(lines) - 10
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}

	// Get the end-of-file offset
	end, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0
	}
	return end
}

// readAllLines reads all lines from a file.
func readAllLines(f *os.File) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
