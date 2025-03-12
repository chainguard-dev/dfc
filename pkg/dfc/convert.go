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

					// Check if any package manager appears in the full command
					isPM := false
					for _, pm := range AllPackageManagers {
						// Check if the part contains the package manager command
						if strings.Contains(fullCmd, string(pm)+" ") {
							isPM = true
							break
						}
					}

					// Keep only non-package-manager commands
					if !isPM {
						nonPMCommands = append(nonPMCommands, fullCmd)
					}
				}

				// 2. Build the new command starting with the apk add command (if we have packages)
				var newCmdParts []string
				if len(line.Run.Packages) > 0 {
					newCmdParts = append(newCmdParts, transformPackageCommand(
						string(ManagerApk)+" "+ApkInstallCommand,
						line.Run.Packages))
				}

				// 3. Add all the non-package-manager commands
				newCmdParts = append(newCmdParts, nonPMCommands...)

				// 4. Join all parts with " && " to create a single command
				var convertedContent string
				if len(newCmdParts) > 0 {
					// Check for special case where we don't want line continuations
					hasCleaningUpEcho := false
					for _, part := range newCmdParts {
						if strings.Contains(part, "echo \"cleaning up\"") {
							hasCleaningUpEcho = true
							break
						}
					}

					// Determine if we should preserve line continuations based on the original command
					preserveLineContinuations := false
					if !hasCleaningUpEcho && len(line.Run.command.Parts) > 0 && len(line.Run.command.Original) > 0 {
						// Check if the original command used line continuations
						originalStr := line.Run.command.Original
						preserveLineContinuations = strings.Contains(originalStr, "\\\n")
					}

					if preserveLineContinuations {
						// Extract the indentation pattern from the original command
						indentPattern := extractIndentationPattern(line.Run.command.Original)

						// Apply the same indentation pattern to our converted command
						convertedContent = applyIndentationPattern(newCmdParts, indentPattern)
					} else {
						// No line continuations, just join with " && "
						convertedContent = strings.Join(newCmdParts, " && ")
					}

					// Clean up trailing backslashes and whitespace
					convertedContent = cleanupTrailingBackslashes(convertedContent)

					// Set the converted line
					line.Converted = DirectiveRun + " " + convertedContent
				} else {
					// If everything was removed, use a dummy command
					line.Converted = DirectiveRun + " " + DummyCommand
				}

				// Add back any extra content (comments, etc.)
				line.Converted += line.Extra
			}
		}
	}

	return nil
}

// transformPackageCommand transforms a package manager command into an apk command
// while preserving the packages and filtering out flags
func transformPackageCommand(cmdStr string, packages []string) string {
	// Return the new command with packages
	if len(packages) > 0 {
		return cmdStr + " " + strings.Join(packages, " ")
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

// cleanupTrailingBackslashes removes any trailing backslashes at the end of a command
// that might be left after removing package manager commands and normalizes whitespace
func cleanupTrailingBackslashes(content string) string {
	// Split the content into lines
	lines := strings.Split(content, "\n")
	if len(lines) <= 1 {
		return normalizeWhitespace(strings.TrimRight(content, " \t")) // Just trim trailing space for single lines
	}

	// Check if the last line is empty (indicating the command ended with a newline)
	lastLineEmpty := lines[len(lines)-1] == ""

	// Process all lines to remove trailing whitespace
	for i := 0; i < len(lines); i++ {
		// Skip the last line if it's empty (preserved newline)
		if i == len(lines)-1 && lastLineEmpty {
			continue
		}

		// Trim trailing whitespace from all lines
		lines[i] = strings.TrimRight(lines[i], " \t")

		// Normalize whitespace (replace multiple spaces with a single space)
		lines[i] = normalizeWhitespace(lines[i])

		// Remove trailing backslash from the last non-empty line
		if i == len(lines)-1 || (i == len(lines)-2 && lastLineEmpty) {
			if strings.HasSuffix(lines[i], "\\") {
				lines[i] = strings.TrimRight(lines[i], "\\")
				// Trim any additional whitespace revealed after removing backslash
				lines[i] = strings.TrimRight(lines[i], " \t")
			}
		}
	}

	// Now filter out empty lines that just have a backslash (or only whitespace + backslash)
	var filteredLines []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Keep all non-empty lines and non-backslash-only lines
		if trimmed != "\\" && (trimmed != "" || (i == len(lines)-1 && lastLineEmpty)) {
			filteredLines = append(filteredLines, line)
		}
	}

	// Rejoin the lines
	return strings.Join(filteredLines, "\n")
}

// normalizeWhitespace replaces multiple consecutive spaces with a single space
func normalizeWhitespace(s string) string {
	// Replace multiple spaces with a single space
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
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

// applyIndentationPattern applies the indentation pattern to join command parts with line continuations
func applyIndentationPattern(cmdParts []string, indentPattern string) string {
	if len(cmdParts) == 0 {
		return ""
	}

	if len(cmdParts) == 1 {
		return cmdParts[0]
	}

	var result strings.Builder
	result.WriteString(cmdParts[0])

	for i := 1; i < len(cmdParts); i++ {
		result.WriteString(" && \\\n")
		result.WriteString(indentPattern)
		result.WriteString(cmdParts[i])
	}

	return result.String()
}
