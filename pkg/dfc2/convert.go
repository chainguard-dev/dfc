package dfc2

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chainguard-dev/dfc/internal/shellparse2"
)

// Use the shellparse2 Node type
type Node = shellparse2.Node

// ConvertDockerfile converts a Dockerfile to use Alpine
func ConvertDockerfile(ctx context.Context, content []byte, opts Options) ([]byte, error) {
	// Parse the Dockerfile
	dockerfile, err := ParseDockerfile(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Dockerfile: %w", err)
	}

	// Apply the conversion
	err = applyConversion(dockerfile, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to apply conversion: %w", err)
	}

	// Rebuild the Dockerfile
	result := rebuildDockerfile(dockerfile)
	return []byte(result), nil
}

// DebugConvertDockerfile converts a Dockerfile to use Alpine with debug output
func DebugConvertDockerfile(ctx context.Context, content []byte, opts Options) ([]byte, error) {
	// Parse the Dockerfile
	dockerfile, err := ParseDockerfile(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Dockerfile: %w", err)
	}

	// Print debug info
	fmt.Fprintf(os.Stderr, "Parsed %d lines\n", len(dockerfile.Lines))
	for i, line := range dockerfile.Lines {
		fmt.Fprintf(os.Stderr, "Line %d: Directive=%s, Raw=%s\n", i, line.Directive, line.Raw)

		if line.Directive == DirectiveRun && line.Run != nil {
			fmt.Fprintf(os.Stderr, "  Run: Distro=%s, Packages=%v\n", line.Run.Distro, line.Run.Packages)

			if line.Run.Command != nil {
				fmt.Fprintf(os.Stderr, "  Command: %s\n", line.Run.Command.String())

				// Find package manager commands
				for _, pm := range AllPackageManagers {
					pmStr := string(pm)
					cmds := line.Run.Command.FindCommandsByPrefix(pmStr)
					if len(cmds) > 0 {
						fmt.Fprintf(os.Stderr, "  Found %d %s commands\n", len(cmds), pmStr)

						for j, cmd := range cmds {
							fmt.Fprintf(os.Stderr, "    Command %d: %s %s\n", j, cmd.Command, strings.Join(cmd.Args, " "))
						}
					}
				}
			}
		}
	}

	// Apply the conversion
	err = applyConversion(dockerfile, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to apply conversion: %w", err)
	}

	// Print debug info after conversion
	fmt.Fprintf(os.Stderr, "\nAfter conversion:\n")
	for i, line := range dockerfile.Lines {
		fmt.Fprintf(os.Stderr, "Line %d: Directive=%s, Raw=%s\n", i, line.Directive, line.Raw)
	}

	// Rebuild the Dockerfile
	result := rebuildDockerfile(dockerfile)
	return []byte(result), nil
}

// applyConversion applies the conversion to the parsed Dockerfile
func applyConversion(dockerfile *Dockerfile, opts Options) error {
	// Process each line
	for _, line := range dockerfile.Lines {
		// Only process FROM and RUN directives
		switch line.Directive {
		case DirectiveFrom:
			convertFromDirective(line, opts, dockerfile.StageAliases)
		case DirectiveRun:
			convertRunDirective(line, opts)
		}
	}

	return nil
}

// convertFromDirective converts FROM directives to use Alpine
func convertFromDirective(line *DockerfileLine, opts Options, stageAliases map[string]bool) {
	if line.From == nil {
		return
	}

	// Don't modify FROM directives that reference other stages
	// or that have dynamic variables
	if line.From.BaseDynamic || isStageReference(line.From.Base, stageAliases) {
		return
	}

	// Organization is required
	if opts.Organization == "" {
		fmt.Fprintf(os.Stderr, "Warning: Organization is required but not provided, using '%s' as placeholder\n", DefaultOrganization)
		opts.Organization = DefaultOrganization
	}

	// Replace the base image with Alpine using cgr.dev/ORGANIZATION/alpine format
	newBase := fmt.Sprintf("%s/%s/%s", DefaultRegistryDomain, opts.Organization, DefaultBaseImage)

	// Update the line
	line.From.Base = newBase
	line.From.Tag = DefaultImageTag

	newTagStr := ""
	if line.From.Tag != "" {
		newTagStr = ":" + line.From.Tag
	}

	// Check if there's an AS clause
	asClause := ""
	if line.From.Alias != "" {
		asClause = " AS " + line.From.Alias
	}

	// Find where in the raw string to replace
	fromPrefix := "FROM "
	fromIndex := strings.Index(line.Raw, fromPrefix)
	if fromIndex == -1 {
		return
	}

	// Update the raw line
	line.Raw = fmt.Sprintf("%sFROM %s%s%s",
		line.Raw[:fromIndex],
		newBase, newTagStr, asClause,
	)
}

// isStageReference checks if a FROM base is a reference to another build stage
func isStageReference(base string, stageAliases map[string]bool) bool {
	// Simple check: if the base is in our map of stage aliases, it's a reference
	if stageAliases != nil && stageAliases[base] {
		return true
	}

	// No matching stage alias found
	return false
}

// convertRunDirective converts RUN directives to use apk
func convertRunDirective(line *DockerfileLine, opts Options) {
	if line.Run == nil || line.Run.Command == nil {
		return
	}

	// Skip if no packages were found or if the distro is unknown
	if len(line.Run.Packages) == 0 || line.Run.Distro == DistroUnknown {
		return
	}

	// Map packages to Alpine equivalents
	mappedPackages := mapPackages(line.Run.Packages, opts.PackageMap)

	// Find install commands for the detected package managers
	cmdsToReplace := []Node{}

	// Track all package manager commands to handle non-install commands
	allPkgManagerCmds := []Node{}

	// Loop through the relevant package managers for this distro
	if packageManagers, exists := PackageManagerGroups[line.Run.Distro]; exists {
		for _, pm := range packageManagers {
			pmStr := string(pm)
			info := PackageManagerInfoMap[pm]

			// Find all commands for this package manager
			allManagerCmds := line.Run.Command.FindCommandsByPrefix(pmStr)

			// Find only install commands for this package manager
			installCmds := line.Run.Command.FindCommandsByPrefixAndSubcommand(pmStr, info.InstallKeyword)

			// Add install commands to our replace list
			cmdsToReplace = append(cmdsToReplace, installCmds...)

			// Add all package manager commands to our tracking list
			allPkgManagerCmds = append(allPkgManagerCmds, allManagerCmds...)
		}
	}

	// Skip if no commands to replace
	if len(cmdsToReplace) == 0 {
		return
	}

	// Create a map of install command string representations for quick lookup
	installCmdStrings := make(map[string]bool)
	for _, cmd := range cmdsToReplace {
		cmdStr := cmd.Command + " " + strings.Join(cmd.Args, " ")
		installCmdStrings[cmdStr] = true
	}

	// Remove all non-install package manager commands
	// (This is a whitelist approach - only keep install commands)
	for _, cmd := range allPkgManagerCmds {
		cmdStr := cmd.Command + " " + strings.Join(cmd.Args, " ")
		if !installCmdStrings[cmdStr] {
			// If it's not an install command, remove it
			line.Run.Command.RemoveCommand(cmd)
		}
	}

	// Generate the replacement apk command
	apkCmd := fmt.Sprintf("%s add -U %s", DefaultPackageManager, strings.Join(mappedPackages, " "))

	// Replace the first install command with apk add
	if len(cmdsToReplace) > 0 {
		// Replace the first command
		line.Run.Command.ReplaceCommand(cmdsToReplace[0], apkCmd)

		// Remove any additional install commands
		for i := 1; i < len(cmdsToReplace); i++ {
			line.Run.Command.RemoveCommand(cmdsToReplace[i])
		}
	}

	// For multi-line commands, we need to reformat the raw line
	if strings.Contains(line.Raw, "\n") {
		// This is a multi-line command
		lines := strings.Split(line.Raw, "\n")
		if len(lines) > 1 {
			// Get the indentation from the second line
			indent := ""
			for i := 0; i < len(lines[1]); i++ {
				if !isWhitespace(lines[1][i]) {
					break
				}
				indent += string(lines[1][i])
			}

			// Format the new command with the same indentation
			line.Raw = fmt.Sprintf("RUN %s", apkCmd)
		}
	} else {
		// Find where in the raw string to replace
		runPrefix := "RUN "
		runIndex := strings.Index(line.Raw, runPrefix)
		if runIndex == -1 {
			return
		}

		// Update the raw line
		line.Raw = fmt.Sprintf("%sRUN %s",
			line.Raw[:runIndex],
			line.Run.Command.String(),
		)
	}
}

// isWhitespace checks if a character is whitespace
func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t'
}

// mapPackages maps packages from the source distro to Alpine equivalents
func mapPackages(packages []string, packageMap map[string]string) []string {
	result := make([]string, 0, len(packages))

	for _, pkg := range packages {
		// Check if there's a mapping
		if mapped, exists := packageMap[pkg]; exists && mapped != "" {
			result = append(result, mapped)
		} else {
			// Keep the original package if no mapping exists
			result = append(result, pkg)
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	uniqueResult := []string{}

	for _, pkg := range result {
		if !seen[pkg] {
			seen[pkg] = true
			uniqueResult = append(uniqueResult, pkg)
		}
	}

	return uniqueResult
}

// rebuildDockerfile reconstructs the Dockerfile from the parsed representation
func rebuildDockerfile(dockerfile *Dockerfile) string {
	var builder strings.Builder

	for i, line := range dockerfile.Lines {
		// Add any ExtraBefore content
		if line.ExtraBefore != "" {
			builder.WriteString(line.ExtraBefore)
			builder.WriteString("\n")
		}

		builder.WriteString(line.Raw)

		// Add newline unless this is the last line
		if i < len(dockerfile.Lines)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}
