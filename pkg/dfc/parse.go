package dfc

import (
	"context"
	"regexp"
	"strings"

	"github.com/chainguard-dev/dfc/internal/shellparse"
)

// ParseDockerfile parses a Dockerfile into a structured representation
func ParseDockerfile(_ context.Context, content []byte) (*Dockerfile, error) {
	dockerfile := &Dockerfile{
		Lines:        []*DockerfileLine{},
		stageAliases: make(map[string]bool),
	}

	// Stage counter for multi-stage builds
	stage := 0

	// Process the content line by line
	lines := strings.Split(string(content), "\n")

	// Variables to track multi-line commands
	var currentMultiLine *DockerfileLine
	var multiLineContent strings.Builder

	// Variables to track comments and empty lines
	var pendingExtra strings.Builder
	var hasExtraContent bool
	var blankLines []int // Track line numbers of blank lines

	for i, line := range lines {
		lineNum := i + 1
		trimmedLine := strings.TrimSpace(line)

		// If we're in a multi-line command, handle it specially
		if currentMultiLine != nil {
			// Skip comments within multi-line commands
			if strings.HasPrefix(trimmedLine, "#") {
				// Store the comment as is, we'll handle it during extraction
				multiLineContent.WriteString("\n")
				multiLineContent.WriteString(line)
				continue
			}

			// Add this line to the multi-line command
			multiLineContent.WriteString("\n")
			multiLineContent.WriteString(line)

			// Check if the line ends with a continuation character
			if !strings.HasSuffix(trimmedLine, "\\") {
				// This is the end of the multi-line command
				currentMultiLine.Raw = multiLineContent.String()

				// Extract command details for RUN directives
				if currentMultiLine.Directive == DirectiveRun {
					currentMultiLine.Run = extractRun(currentMultiLine.Raw)
				}

				// Add the multi-line command to the Dockerfile
				dockerfile.Lines = append(dockerfile.Lines, currentMultiLine)

				// Reset multi-line tracking
				currentMultiLine = nil
				multiLineContent.Reset()
			}
			continue
		}

		// Handle empty lines and comments
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			// Track blank lines explicitly
			if trimmedLine == "" {
				blankLines = append(blankLines, lineNum)
			}

			// Add to pending extra content
			if hasExtraContent {
				pendingExtra.WriteString("\n")
			}
			pendingExtra.WriteString(line)
			hasExtraContent = true

			continue
		}

		// This is a regular line with a directive
		// Parse the directive
		dfLine := parseDockerfileLine(line)
		if dfLine == nil {
			continue
		}

		// If we have pending blank lines, make sure they're captured
		if len(blankLines) > 0 {
			// Generate explicit newlines based on blank line count
			extraNewlines := strings.Repeat("\n", len(blankLines))
			if hasExtraContent {
				// If we have other content, add it too
				dfLine.Extra = pendingExtra.String() + extraNewlines
			} else {
				dfLine.Extra = extraNewlines
			}
		} else if hasExtraContent {
			// No blank lines but we have extra content
			dfLine.Extra = pendingExtra.String()
		}

		// Reset tracking
		pendingExtra.Reset()
		hasExtraContent = false
		blankLines = nil

		// For FROM directives, update stage info
		if dfLine.Directive == DirectiveFrom {
			stage++
			dfLine.Stage = stage
			if dfLine.From != nil {
				// Record the stage alias if present
				if dfLine.From.Alias != "" {
					dockerfile.stageAliases[dfLine.From.Alias] = true
				}
			}
		} else if dfLine.Directive != "" {
			// For other directives, set the current stage
			dfLine.Stage = stage
		}

		// Check if this is the start of a multi-line command
		if strings.HasSuffix(trimmedLine, "\\") {
			// This is a multi-line command
			currentMultiLine = dfLine
			multiLineContent.WriteString(line)
			continue
		}

		// Extract command details for RUN directives
		if dfLine.Directive == DirectiveRun {
			dfLine.Run = extractRun(dfLine.Raw)
		}

		dockerfile.Lines = append(dockerfile.Lines, dfLine)
	}

	// Handle any remaining multi-line command
	if currentMultiLine != nil {
		currentMultiLine.Raw = multiLineContent.String()

		// Extract command details for RUN directives
		if currentMultiLine.Directive == DirectiveRun {
			currentMultiLine.Run = extractRun(currentMultiLine.Raw)
		}

		dockerfile.Lines = append(dockerfile.Lines, currentMultiLine)
	}

	// Handle any trailing comments/whitespace
	if hasExtraContent {
		// Create a final line with only extra content
		dockerfile.Lines = append(dockerfile.Lines, &DockerfileLine{
			Raw:       "",
			Extra:     pendingExtra.String(),
			Directive: "",
			Stage:     stage,
		})
	}

	return dockerfile, nil
}

// parseDockerfileLine parses a single line from a Dockerfile
func parseDockerfileLine(line string) *DockerfileLine {
	trimmedLine := strings.TrimSpace(line)
	if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
		// Empty line or comment - these should be handled as Extra content
		return nil
	}

	// Check for a directive
	parts := strings.SplitN(trimmedLine, " ", 2)
	directive := strings.ToUpper(parts[0])

	// Verify this is a valid Dockerfile directive
	// Common directives include: FROM, RUN, CMD, LABEL, EXPOSE, ENV, ADD, COPY, ENTRYPOINT, VOLUME, USER, WORKDIR, ARG, ONBUILD, STOPSIGNAL, HEALTHCHECK, SHELL
	// Operators like && are not directives
	isValidDirective := directive == "FROM" || directive == "RUN" || directive == "CMD" ||
		directive == "LABEL" || directive == "EXPOSE" || directive == "ENV" ||
		directive == "ADD" || directive == "COPY" || directive == "ENTRYPOINT" ||
		directive == "VOLUME" || directive == "USER" || directive == "WORKDIR" ||
		directive == "ARG" || directive == "ONBUILD" || directive == "STOPSIGNAL" ||
		directive == "HEALTHCHECK" || directive == "SHELL" || directive == "MAINTAINER"

	// If it's not a valid directive, it might be a continuation of a previous command
	// But since we're parsing line by line, we'll return nil and it should be handled as part of a multi-line command
	if !isValidDirective {
		return nil
	}

	dfLine := &DockerfileLine{
		Raw:       line,
		Directive: directive,
	}

	// Process specific directives
	switch directive {
	case DirectiveFrom:
		if len(parts) > 1 {
			dfLine.From = parseFromDirective(parts[1])
		}
	case DirectiveRun:
		// For RUN commands, we'll extract details later
		if len(parts) > 1 {
			// We keep the entire raw line here
			// We'll extract the command content later in extractRun
		}
	}

	return dfLine
}

// parseFromDirective parses a FROM directive to extract image and stage details
func parseFromDirective(content string) *FromDetails {
	from := &FromDetails{}

	// We'll use a case-insensitive regular expression to find the AS clause
	// This will match " AS " with spaces around it (common case)
	reAS := regexp.MustCompile(`(?i)(\s+` + KeywordAs + `\s+)(.+)$`)
	matches := reAS.FindStringSubmatchIndex(content)

	if len(matches) > 0 {
		// We found an AS clause
		asStart := matches[2]
		aliasStart := matches[4]

		// Extract the alias using the original content to preserve case
		from.Alias = strings.TrimSpace(content[aliasStart:])

		// Trim the AS clause from the content
		content = strings.TrimSpace(content[:asStart])
	}

	// Parse image and tag
	imageParts := strings.Split(content, ":")
	from.Base = strings.TrimSpace(imageParts[0])
	if len(imageParts) > 1 {
		from.Tag = strings.TrimSpace(imageParts[1])

		// Check if the tag itself contains an AS keyword
		tagASMatches := reAS.FindStringSubmatchIndex(from.Tag)

		if len(tagASMatches) > 0 {
			// Extract the actual tag and alias
			asStart := tagASMatches[2]
			aliasStart := tagASMatches[4]

			from.Alias = strings.TrimSpace(from.Tag[aliasStart:])
			from.Tag = strings.TrimSpace(from.Tag[:asStart])
		}
	}

	// Check for dynamic parts (variables)
	from.BaseDynamic = strings.Contains(from.Base, "${") || strings.Contains(from.Base, "$")
	from.TagDynamic = strings.Contains(from.Tag, "${") || strings.Contains(from.Tag, "$")

	return from
}

// extractRun extracts information from a RUN directive
func extractRun(content string) *RunDetails {
	// Create a new RunDetails object
	details := &RunDetails{}

	// Extract the command part (after "RUN ")
	cmdStart := strings.Index(content, "RUN ")
	if cmdStart == -1 {
		return details
	}

	cmdContent := content[cmdStart+4:]

	// Clean up the command content for analysis
	// This is used only for analysis, not for preserving formatting
	analyzableContent := cleanupCommandContent(cmdContent)

	// Parse the shell command with the original content to preserve formatting
	cmd := shellparse.NewShellCommand(cmdContent)
	details.command = cmd

	// Use the analyzable content to determine the package manager
	analyzeCmd := shellparse.NewShellCommand(analyzableContent)

	// First, determine the distribution based on package manager commands
	var detectedPackageManager Manager
	for _, pm := range AllPackageManagers {
		pmStr := string(pm)

		// Check if this package manager is used in the command
		pmCmds := analyzeCmd.FindCommandsByPrefix(pmStr)
		if len(pmCmds) > 0 {
			// Found a package manager, get its associated distro
			info := PackageManagerInfoMap[pm]
			details.Distro = info.Distro
			detectedPackageManager = pm
			break
		}
	}

	// If we arent able to detect distro, just return nil
	if details.Distro == "" {
		return nil
	}

	// Extract packages only for the detected package manager
	if detectedPackageManager != "" {
		pmStr := string(detectedPackageManager)
		info := PackageManagerInfoMap[detectedPackageManager]

		// Find install commands for this package manager
		installCmds := analyzeCmd.FindCommandsByPrefixAndSubcommand(pmStr, info.InstallKeyword)

		// Extract packages from each install command
		for _, installCmd := range installCmds {
			packages := shellparse.ExtractPackagesFromInstallCommand(installCmd)
			details.Packages = append(details.Packages, packages...)
		}
	}

	// Deduplicate packages
	if len(details.Packages) > 0 {
		details.Packages = deduplicatePackages(details.Packages)
	}

	return details
}

// cleanupCommandContent removes comments and normalizes line breaks in multi-line commands
// This is used only for analysis, not for preserving formatting
func cleanupCommandContent(content string) string {
	lines := strings.Split(content, "\n")
	var cleanedLines []string
	var currentCommand string

	for _, line := range lines {
		// Strip leading/trailing whitespace
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		// Check if this is a continuation or a new command
		if strings.HasPrefix(trimmedLine, "&&") || strings.HasPrefix(trimmedLine, "||") ||
			strings.HasPrefix(trimmedLine, ";") {
			// This is a continuation, append it to the current command
			currentCommand += " " + trimmedLine
		} else if strings.HasSuffix(trimmedLine, "\\") {
			// This is a continuation with backslash, remove the backslash and add to current command
			trimmedLine = strings.TrimSuffix(trimmedLine, "\\")
			if currentCommand == "" {
				currentCommand = trimmedLine
			} else {
				currentCommand += " " + trimmedLine
			}
		} else {
			// This is a new command
			if currentCommand != "" {
				// Add the previous command
				cleanedLines = append(cleanedLines, currentCommand)
			}
			currentCommand = trimmedLine
		}
	}

	// Add the last command if there is one
	if currentCommand != "" {
		cleanedLines = append(cleanedLines, currentCommand)
	}

	// Join with spaces to normalize the command
	return strings.Join(cleanedLines, " ")
}

// deduplicatePackages removes duplicate packages from a slice
func deduplicatePackages(packages []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, pkg := range packages {
		if !seen[pkg] {
			seen[pkg] = true
			result = append(result, pkg)
		}
	}

	return result
}
