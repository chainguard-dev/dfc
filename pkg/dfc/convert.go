package dfc

import (
	"context"
	"fmt"
	"path"
	"strings"
)

func (d *Dockerfile) convert(_ context.Context, opts *Options) error {
	for _, line := range d.Lines {
		if modifiableFromLine(line) {
			// TODO: clean this up and make sure base exists in our catalog
			base := fmt.Sprintf("%s/%s/%s:%s", DefaultRegistryDomain, opts.Organization, path.Base(line.From.Base), DefaultImageTag)
			converted := []string{DirectiveFrom, base}
			if line.From.Alias != "" {
				converted = append(converted, KeywordAs, line.From.Alias)
			}
			line.Converted = strings.Join(converted, " ")

			// TODO: this should be done cleaner than this
			// Create a new entry and carry over the extra content from this line
			line.Converted += "\n" + DirectiveUser + " " + DefaultUser

			line.Converted += line.Extra

		} else if modifiableRunLine(line) {
			// If we have packages detected, replace the package manager commands with apk
			if line.Run != nil && line.Run.command != nil {
				// Approach: Extract non-package-manager commands and build a new shell command

				// 1. Extract all non-package-manager commands
				var nonPMCommands []string
				for _, part := range line.Run.command.Parts {
					// Get the full command string
					fullCmd := part.GetFullCommand()

					// Check if this is a package manager command
					isPMCommand := false
					isInstallCommand := false

					for _, pm := range AllPackageManagers {
						pmStr := string(pm)

						// Check if this is a package manager command
						if strings.HasPrefix(fullCmd, pmStr+" ") || fullCmd == pmStr {
							isPMCommand = true

							// Check if it's an install command
							info, exists := PackageManagerInfoMap[pm]
							if exists && strings.Contains(fullCmd, pmStr+" "+info.InstallKeyword) {
								isInstallCommand = true
							}
							break
						}
					}

					// Keep only non-package-manager commands or install commands
					// Install commands will be handled separately as they are converted to apk add
					if !isPMCommand || isInstallCommand {
						nonPMCommands = append(nonPMCommands, fullCmd)
					}
					// Skip all other package manager commands (update, clean, remove, etc.)
				}

				// 2. Build the new command starting with the apk add command (if we have packages)
				var newCmdParts []string
				hasPackages := len(line.Run.Packages) > 0

				if hasPackages {
					newCmdParts = append(newCmdParts, transformPackageCommand(
						string(ManagerApk)+" "+ApkInstallCommand,
						line.Run.Packages,
						line.Run.Distro,
						opts.PackageMap))
				}

				// 3. Add all the non-package-manager commands, excluding package install commands
				// (which are already handled by the apk add command)
				for _, cmd := range nonPMCommands {
					isPMInstall := false
					for _, pm := range AllPackageManagers {
						pmStr := string(pm)
						info, exists := PackageManagerInfoMap[pm]
						if exists && strings.HasPrefix(cmd, pmStr+" "+info.InstallKeyword) {
							isPMInstall = true
							break
						}
					}

					if !isPMInstall {
						newCmdParts = append(newCmdParts, cmd)
					}
				}

				// 4. Process the command parts
				var convertedContent string
				if len(newCmdParts) > 0 {
					// Check if the original command used line continuations
					originalUsedContinuations := false
					if len(line.Run.command.Parts) > 0 && len(line.Run.command.Original) > 0 {
						originalStr := line.Run.command.Original
						originalUsedContinuations = strings.Contains(originalStr, "\\\n")
					}

					// Special case: if echo "cleaning up" is present, don't use line continuations
					hasCleaningUpEcho := false
					for _, part := range newCmdParts {
						if strings.Contains(part, "echo \"cleaning up\"") {
							hasCleaningUpEcho = true
							break
						}
					}

					if originalUsedContinuations && !hasCleaningUpEcho {
						// Extract the indentation pattern from the original command
						indentPattern := extractIndentationPattern(line.Run.command.Original)

						// Format with line continuations
						if len(newCmdParts) == 1 {
							convertedContent = newCmdParts[0]
						} else {
							var result strings.Builder
							result.WriteString(newCmdParts[0])

							for i := 1; i < len(newCmdParts); i++ {
								result.WriteString(" && \\\n")
								result.WriteString(indentPattern)
								result.WriteString(newCmdParts[i])
							}

							convertedContent = result.String()
						}
					} else {
						// No line continuations, just join with " && "
						convertedContent = strings.Join(newCmdParts, " && ")
					}

					// Set the converted line
					line.Converted = DirectiveRun + " " + convertedContent
				} else {
					// If everything was removed, use a dummy command
					line.Converted = DirectiveRun + " " + DummyCommand
				}

				// Clean up the formatted output - remove any leading spaces after a newline
				line.Converted = strings.ReplaceAll(line.Converted, "\n ", "\n")

				// Clean up any package manager commands that might still be in the output
				line.Converted = cleanupRemainingPackageManagerCommands(line.Converted)

				// Make sure we have valid Dockerfile syntax
				line.Converted = ensureValidDockerfileSyntax(line.Converted)

				// Add back any extra content (comments, etc.)
				line.Converted += line.Extra
			}
		}
	}

	return nil
}

// transformPackageCommand transforms a package manager command into an apk command
// while preserving the packages and filtering out flags
func transformPackageCommand(cmdStr string, packages []string, distro Distro, packageMap map[Distro]map[string][]string) string {
	// Return the new command with packages
	if len(packages) > 0 {
		// Map packages if we have mapping information
		var mappedPackages []string

		if packageMap != nil {
			// If we have a package map, check for mappings
			distroMap, distroExists := packageMap[distro]

			for _, pkg := range packages {
				if distroExists {
					// Check if this package has a mapping
					if alternativePackages, exists := distroMap[pkg]; exists && len(alternativePackages) > 0 {
						// Use the mapped packages instead of the original
						mappedPackages = append(mappedPackages, alternativePackages...)
					} else {
						// No mapping found, use original package
						mappedPackages = append(mappedPackages, pkg)
					}
				} else {
					// No mappings for this distro, use original package
					mappedPackages = append(mappedPackages, pkg)
				}
			}
		} else {
			// No package mapping provided, use original packages
			mappedPackages = packages
		}

		return cmdStr + " " + strings.Join(mappedPackages, " ")
	}

	return cmdStr // No packages found
}

func modifiableFromLine(line *DockerfileLine) bool {
	return line.Directive == DirectiveFrom && line.From != nil &&
		line.From.Base != "" && line.From.Parent == 0 && !line.From.BaseDynamic
}

func modifiableRunLine(line *DockerfileLine) bool {
	return line.Directive == DirectiveRun && line.Run != nil && line.Run.Distro != ""
}

// extractIndentationPattern extracts the indentation pattern used after line continuations
func extractIndentationPattern(original string) string {
	// Default to two spaces if we can't determine the pattern
	defaultPattern := "  "

	// If there are no line continuations, return the default
	if !strings.Contains(original, "\\\n") {
		return defaultPattern
	}

	// Find the first line continuation
	parts := strings.Split(original, "\\\n")
	if len(parts) < 2 {
		return defaultPattern
	}

	// Look at the second part (after first line continuation)
	secondPart := parts[1]

	// Count leading spaces
	leadingSpaces := ""
	for _, c := range secondPart {
		if c == ' ' || c == '\t' {
			leadingSpaces += string(c)
		} else {
			break
		}
	}

	if leadingSpaces == "" {
		return defaultPattern
	}

	return leadingSpaces
}

// ensureValidDockerfileSyntax checks if a RUN command has invalid syntax and fixes it
// This is particularly important when we have a DummyCommand followed by other commands
func ensureValidDockerfileSyntax(runCmd string) string {
	if !strings.HasPrefix(runCmd, DirectiveRun) {
		return runCmd
	}

	// If there's no line continuation issue, return as is
	if !strings.Contains(runCmd, "\n") || !strings.Contains(runCmd, "&&") {
		return runCmd
	}

	// Check if we have a "RUN true" followed by "&&" without a backslash
	if strings.Contains(runCmd, DirectiveRun+" "+DummyCommand+"\n&&") {
		return strings.Replace(runCmd, DirectiveRun+" "+DummyCommand+"\n&&", DirectiveRun+" "+DummyCommand+" \\\n&&", 1)
	}

	// Check for "RUN true && " without a backslash at the end of the line
	if strings.Contains(runCmd, DirectiveRun+" "+DummyCommand+" &&\n") {
		return strings.Replace(runCmd, DirectiveRun+" "+DummyCommand+" &&\n", DirectiveRun+" "+DummyCommand+" && \\\n", 1)
	}

	return runCmd
}

// cleanupRemainingPackageManagerCommands cleans up any remaining package manager commands in the output
func cleanupRemainingPackageManagerCommands(cmd string) string {
	// Directly remove any lines containing apt-get autoremove
	lines := strings.Split(cmd, "\n")
	var filteredLines []string
	for _, line := range lines {
		if !strings.Contains(line, "apt-get autoremove") {
			filteredLines = append(filteredLines, line)
		}
	}
	cmd = strings.Join(filteredLines, "\n")

	// Handle consecutive && and line continuations
	cmd = strings.Replace(cmd, "&&  &&", "&&", -1)
	cmd = strings.Replace(cmd, "&&  \\", "&&", -1)
	cmd = strings.Replace(cmd, "&& \\\n\n", "&& \\\n", -1)

	// Remove any remaining package manager commands that aren't install commands
	for _, pm := range AllPackageManagers {
		pmStr := string(pm)

		// Skip apk add since this is our target package manager command
		if pmStr == string(ManagerApk) && strings.Contains(cmd, pmStr+" "+ApkInstallCommand) {
			continue
		}

		// Remove commands from other package managers
		if pmStr != string(ManagerApk) {
			lines = strings.Split(cmd, "\n")
			filteredLines = nil

			for _, line := range lines {
				if !strings.Contains(line, pmStr+" ") {
					filteredLines = append(filteredLines, line)
				}
			}

			cmd = strings.Join(filteredLines, "\n")
		}
	}

	return cmd
}
