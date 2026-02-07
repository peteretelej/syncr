package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// prompt prints a label and reads one line of input. Returns the trimmed string.
// Returns empty string on EOF (pipe closed).
func prompt(label string) string {
	fmt.Print(label)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

// confirm prints a label with [Y/n] and returns true for empty/y/Y, false otherwise.
func confirm(label string) bool {
	fmt.Printf("%s [Y/n] ", label)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		return input == "" || strings.EqualFold(input, "y") || strings.EqualFold(input, "yes")
	}
	return false
}
