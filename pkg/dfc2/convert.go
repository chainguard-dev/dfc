package dfc2

import (
	"context"
	"fmt"
	"os"
	"regexp"
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
	convertedDockerfile := dockerfile.Convert(ctx, opts)

	// Return the string representation
	return []byte(convertedDockerfile.String()), nil
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
		asClause = " " + KeywordAs + " " + line.From.Alias
	}

	// Find where in the raw string to replace
	fromPrefix := DirectiveFrom + " "
	fromIndex := strings.Index(line.Raw, fromPrefix)
	if fromIndex == -1 {
		return
	}

	// Update the raw line
	line.Raw = fmt.Sprintf("%s%s %s%s%s",
		line.Raw[:fromIndex],
		DirectiveFrom, newBase, newTagStr, asClause,
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

// convertRunDirective converts a RUN directive in a Dockerfile
func convertRunDirective(line *DockerfileLine, opts Options) {
	var distro Distro
	if line.Run != nil {
		distro = line.Run.Distro
	}

	// Get information about package managers for the detected distro
	distroPackageManagers, found := PackageManagerGroups[distro]
	if !found || len(distroPackageManagers) == 0 {
		// No package managers for this distro, leave as-is
		return
	}

	// Find all package manager commands in the RUN line
	var allManagerCmds []Node
	var installCmds []Node
	for _, pm := range distroPackageManagers {
		pmStr := string(pm)
		info := PackageManagerInfoMap[pm]

		// Find all commands for this package manager
		allManagerCmds = append(allManagerCmds, line.Run.Command.FindCommandsByPrefix(pmStr)...)

		// Find install commands for this package manager
		installCmds = append(installCmds, line.Run.Command.FindCommandsByPrefixAndSubcommand(pmStr, info.InstallKeyword)...)
	}

	// If no package manager commands found, leave as-is
	if len(allManagerCmds) == 0 {
		return
	}

	// Get the list of packages to install
	packages := line.Run.Packages
	if len(packages) == 0 {
		// No packages detected, remove all package manager commands
		for _, cmd := range allManagerCmds {
			line.Run.Command.RemoveCommand(cmd)
		}
		rebuildRawRunLine(line)
		return
	}

	// Apply package mapping if provided
	if opts.PackageMap != nil {
		for i, pkg := range packages {
			if mappedPkg, found := opts.PackageMap[pkg]; found {
				packages[i] = mappedPkg
			}
		}
	}

	// Create a new apk command to install packages
	pkgList := strings.Join(packages, " ")
	apkCmd := DefaultInstallCommand + " " + pkgList

	// Decide which command to replace/remove
	if len(installCmds) > 0 {
		// If we have install commands, replace the first one
		line.Run.Command.ReplaceCommand(installCmds[0], apkCmd)

		// Create a map to track which commands we've already processed
		processedCmds := make(map[string]bool)
		processedCmds[cmdToString(installCmds[0])] = true

		// Remove any additional install commands
		for i := 1; i < len(installCmds); i++ {
			processedCmds[cmdToString(installCmds[i])] = true
			line.Run.Command.RemoveCommand(installCmds[i])
		}

		// Also remove ALL other package manager commands (not just install ones)
		for _, cmd := range allManagerCmds {
			// Skip if we've already processed this command
			if !processedCmds[cmdToString(cmd)] {
				line.Run.Command.RemoveCommand(cmd)
			}
		}
	} else if len(allManagerCmds) > 0 {
		// If no install commands but we have package manager commands,
		// replace the first package manager command
		line.Run.Command.ReplaceCommand(allManagerCmds[0], apkCmd)

		// Remove any additional package manager commands
		for i := 1; i < len(allManagerCmds); i++ {
			line.Run.Command.RemoveCommand(allManagerCmds[i])
		}
	}

	rebuildRawRunLine(line)
}

// rebuildRawRunLine rebuilds the raw line for a RUN directive
func rebuildRawRunLine(line *DockerfileLine) {
	// Get the command string
	cmdStr := line.Run.Command.String()

	// Fix spacing around operators
	cmdStr = strings.ReplaceAll(cmdStr, "&&", " && ")
	cmdStr = strings.ReplaceAll(cmdStr, "||", " || ")

	// Remove leading && or || operators that might be left if we removed the first command
	cmdStr = strings.TrimSpace(cmdStr)
	if strings.HasPrefix(cmdStr, "&&") {
		cmdStr = strings.TrimSpace(cmdStr[2:])
	} else if strings.HasPrefix(cmdStr, "||") {
		cmdStr = strings.TrimSpace(cmdStr[2:])
	}

	// Clean up any double spaces that might have been introduced
	for strings.Contains(cmdStr, "  ") {
		cmdStr = strings.ReplaceAll(cmdStr, "  ", " ")
	}

	// Clean up any double operators that might be introduced by removing commands in the middle
	cmdStr = strings.ReplaceAll(cmdStr, "&& &&", "&&")
	cmdStr = strings.ReplaceAll(cmdStr, "|| ||", "||")

	// Update the raw line
	line.Raw = DirectiveRun + " " + cmdStr

	// Post-processing: Clean up any remaining package manager commands
	// This is a fallback in case the shell parser missed some commands
	for _, pmGroup := range PackageManagerGroups {
		for _, pm := range pmGroup {
			pmStr := string(pm)
			// Remove any remaining package manager commands from the raw line
			if strings.Contains(line.Raw, pmStr) {
				// First, handle the case where the package manager command is at the beginning of the line
				if strings.Contains(line.Raw, DirectiveRun+" "+pmStr) || strings.Contains(line.Raw, DirectiveRun+"\n"+pmStr) {
					re := regexp.MustCompile(`(?i)` + DirectiveRun + `\s+` + pmStr + `\s+\w+(?:\s+[^&|;]+)?(\s*&&|\s*\|\|)?`)
					line.Raw = re.ReplaceAllString(line.Raw, DirectiveRun+"$1")
				}

				// Then handle package manager commands in the middle of the line
				re := regexp.MustCompile(`(?i)(\s*&&|\s*\|\|)?\s*` + pmStr + `\s+\w+(?:\s+[^&|;]+)?(\s*&&|\s*\|\|)?`)
				line.Raw = re.ReplaceAllString(line.Raw, "$1$2")

				// Clean up any double operators
				line.Raw = strings.ReplaceAll(line.Raw, "&& &&", "&&")
				line.Raw = strings.ReplaceAll(line.Raw, "|| ||", "||")
				line.Raw = strings.ReplaceAll(line.Raw, "&&  &&", "&&")
				line.Raw = strings.ReplaceAll(line.Raw, "||  ||", "||")
				line.Raw = strings.ReplaceAll(line.Raw, "&&&&", "&&")
				line.Raw = strings.ReplaceAll(line.Raw, "||||", "||")

				// Remove leading operators
				line.Raw = strings.Replace(line.Raw, DirectiveRun+" &&", DirectiveRun, 1)
				line.Raw = strings.Replace(line.Raw, DirectiveRun+" ||", DirectiveRun, 1)

				// Clean up spaces
				for strings.Contains(line.Raw, "  ") {
					line.Raw = strings.ReplaceAll(line.Raw, "  ", " ")
				}
			}
		}
	}

	// Remove trailing operators (&&, ||) that might be left after removing commands at the end
	line.Raw = strings.TrimSpace(line.Raw)
	if strings.HasSuffix(line.Raw, "&&") {
		line.Raw = strings.TrimSuffix(line.Raw, "&&")
		line.Raw = strings.TrimSpace(line.Raw)
	} else if strings.HasSuffix(line.Raw, "||") {
		line.Raw = strings.TrimSuffix(line.Raw, "||")
		line.Raw = strings.TrimSpace(line.Raw)
	}

	// Also handle multiline cases with trailing operators on the last line
	lines := strings.Split(line.Raw, "\n")
	if len(lines) > 1 {
		lastIdx := len(lines) - 1
		lastLine := strings.TrimSpace(lines[lastIdx])
		if lastLine == "&&" || lastLine == "||" {
			// Remove the last line if it's just an operator
			lines = lines[:lastIdx]
			line.Raw = strings.Join(lines, "\n")
		} else if strings.HasSuffix(lastLine, " &&") || strings.HasSuffix(lastLine, " ||") {
			// Remove trailing operator from the last line
			lines[lastIdx] = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(lastLine, " &&"), " ||"))
			line.Raw = strings.Join(lines, "\n")
		}

		// Also handle the case where we have trailing backslashes
		// After removing trailing commands, we might end up with a backslash on the second-to-last line
		if len(lines) > 1 {
			lastIdx = len(lines) - 1
			secondLastIdx := lastIdx - 1

			// Check if the last line is now empty or very minimal after removing commands
			if strings.TrimSpace(lines[lastIdx]) == "" || strings.TrimSpace(lines[lastIdx]) == "\\" {
				// Remove the last line completely
				lines = lines[:lastIdx]

				// And remove backslash from the new last line if it exists
				if len(lines) > 0 {
					newLastLine := strings.TrimRight(lines[len(lines)-1], " \t")
					if strings.HasSuffix(newLastLine, "\\") {
						lines[len(lines)-1] = strings.TrimSuffix(newLastLine, "\\")
					}
				}

				line.Raw = strings.Join(lines, "\n")
			} else {
				// If the last line has content but the line before it ends with a backslash
				if secondLastIdx >= 0 {
					secondLastLine := strings.TrimRight(lines[secondLastIdx], " \t")
					if strings.HasSuffix(secondLastLine, "\\") {
						// Keep the backslash as it's needed for continuation
						// Nothing to change
					}
				}
			}
		}
	}

	// Final cleanup for any trailing backslashes or characters at the end of the entire raw command
	trimmed := strings.TrimSpace(line.Raw)
	if strings.HasSuffix(trimmed, "\\") {
		// Remove trailing backslash from the last line
		line.Raw = strings.TrimSpace(strings.TrimSuffix(trimmed, "\\"))
	} else if strings.HasSuffix(trimmed, "%") {
		// Remove trailing % character (sometimes added by shell output)
		line.Raw = strings.TrimSpace(strings.TrimSuffix(trimmed, "%"))
	}

	// Clean up any % characters that might be at the end of words
	linesTmp := strings.Split(line.Raw, "\n")
	for i, l := range linesTmp {
		// Check for % in the line
		if strings.Contains(l, "%") {
			words := strings.Fields(l)
			for j, word := range words {
				if strings.HasSuffix(word, "%") {
					words[j] = strings.TrimSuffix(word, "%")
				}
			}
			linesTmp[i] = strings.Join(words, " ")
		}
	}
	line.Raw = strings.Join(linesTmp, "\n")

	// Final cleanup: if the RUN line is empty or just has operators, add the apk command back
	trimmedRaw := strings.TrimSpace(line.Raw)
	if trimmedRaw == DirectiveRun || trimmedRaw == DirectiveRun+" &&" || trimmedRaw == DirectiveRun+" ||" || strings.HasPrefix(trimmedRaw, DirectiveRun+"&&") {
		// If we have packages, add them back
		if len(line.Run.Packages) > 0 {
			pkgList := strings.Join(line.Run.Packages, " ")

			// Check if there are other commands after the package manager commands
			// If so, preserve them
			otherCmds := ""

			// Get the raw command string
			rawCmdStr := line.Run.Command.String()

			// Find all non-package-manager commands
			var nonPMCmds []string
			cmdParts := strings.Split(rawCmdStr, "&&")
			for _, part := range cmdParts {
				part = strings.TrimSpace(part)
				// Skip empty parts
				if part == "" {
					continue
				}

				// Check if this is a package manager command
				isPMCmd := false
				for _, pmGroup := range PackageManagerGroups {
					for _, pm := range pmGroup {
						pmStr := string(pm)
						if strings.HasPrefix(part, pmStr) || strings.Contains(part, " "+pmStr+" ") {
							isPMCmd = true
							break
						}
					}
					if isPMCmd {
						break
					}
				}

				// If not a package manager command, add it to our list
				if !isPMCmd {
					nonPMCmds = append(nonPMCmds, part)
				}
			}

			// Build the final command
			if len(nonPMCmds) > 0 {
				otherCmds = " && " + strings.Join(nonPMCmds, " && ")
			}

			line.Raw = DirectiveRun + " " + DefaultInstallCommand + " " + pkgList + otherCmds
		} else {
			// If no packages and no other commands, use "true" as a no-op
			if strings.TrimSpace(line.Raw) == DirectiveRun || strings.TrimSpace(line.Raw) == DirectiveRun+" " {
				line.Raw = DirectiveRun + " true"
			} else if strings.Contains(line.Raw, "&&") || strings.Contains(line.Raw, "||") {
				// Extract the part after the operators
				re := regexp.MustCompile(DirectiveRun + `\s*(?:&&|\|\|)\s*(.*)`)
				matches := re.FindStringSubmatch(line.Raw)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					// If there's content after the operators, use it
					line.Raw = DirectiveRun + " " + matches[1]
				} else {
					// If nothing after the operators either, use "true"
					line.Raw = DirectiveRun + " true"
				}
			}
		}
	} else if strings.Contains(line.Raw, DefaultInstallCommand) {
		// If we already have an apk add command, make sure we don't add another one
		// This is to prevent duplicate apk add commands
		return
	}
}

// cmdToString creates a string representation of a Node that can be used as a map key
func cmdToString(cmd shellparse2.Node) string {
	return cmd.Command + ":" + strings.Join(cmd.Args, ",")
}
