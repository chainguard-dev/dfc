package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chainguard-dev/dfc/internal/shellparse2"
)

func debugShellParser() {
	if len(os.Args) < 3 || os.Args[1] != "debug" {
		return
	}

	// Get the command from the command line
	cmd := os.Args[2]

	// Parse the command
	shellCmd := shellparse2.NewShellCommand(cmd)

	// Print the parsed command
	fmt.Printf("Original: %s\n", cmd)
	fmt.Printf("Parsed: %s\n", shellCmd.String())

	// Find apt-get commands
	aptGetCmds := shellCmd.FindCommandsByPrefix("apt-get")
	fmt.Printf("Found %d apt-get commands\n", len(aptGetCmds))

	for i, aptGetCmd := range aptGetCmds {
		fmt.Printf("  Command %d: %s\n", i+1, aptGetCmd.Raw)

		// If it's an install command, extract packages
		if len(aptGetCmd.Args) > 0 && aptGetCmd.Args[0] == "install" {
			packages := shellparse2.ExtractPackagesFromInstallCommand(aptGetCmd)
			fmt.Printf("    Packages: %v\n", packages)

			// Replace with apk
			shellCmd.ReplaceCommand(aptGetCmd, fmt.Sprintf("apk add -U %s", strings.Join(packages, " ")))
		} else if len(aptGetCmd.Args) > 0 && aptGetCmd.Args[0] == "update" {
			// Remove update commands
			shellCmd.RemoveCommand(aptGetCmd)
		}
	}

	// Print the modified command
	fmt.Printf("Modified: %s\n", shellCmd.String())

	os.Exit(0)
}
