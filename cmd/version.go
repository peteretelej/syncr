package cmd

import "fmt"

// Version prints the version and commit hash.
func Version(version, commit string) {
	fmt.Printf("syncr %s (%s)\n", version, commit)
}
