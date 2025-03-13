package dfc2

import (
	"context"
	"strings"

	"github.com/chainguard-dev/dfc/internal/shellparse2"
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
				dfLine.ExtraBefore = pendingExtra.String() + extraNewlines
			} else {
				dfLine.ExtraBefore = extraNewlines
			}
		} else if hasExtraContent {
			// No blank lines but we have extra content
			dfLine.ExtraBefore = pendingExtra.String()
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
				if dfLine.From.Parent > 0 {
					// Link to parent stage
					dfLine.From.Parent = dfLine.From.Parent
				}
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
			Raw:         pendingExtra.String(),
			ExtraBefore: "",
			Directive:   "",
			Stage:       stage,
		})
	}

	return dockerfile, nil
}

// parseDockerfileLine parses a single line from a Dockerfile
func parseDockerfileLine(line string) *DockerfileLine {
	trimmedLine := strings.TrimSpace(line)
	if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
		// Empty line or comment - these should be handled as ExtraBefore content
		return nil
	}

	// Check for a directive
	parts := strings.SplitN(trimmedLine, " ", 2)
	directive := strings.ToUpper(parts[0])

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

	// Look for AS clause for stage name
	asParts := strings.Split(content, " AS ")
	if len(asParts) > 1 {
		content = asParts[0]
		from.Alias = strings.TrimSpace(asParts[1])
	}

	// Check if referencing another stage
	if !strings.Contains(content, "/") && !strings.Contains(content, ":") {
		// Could be a reference to another stage
		// For simplicity, we'll just set the base and check later
		from.Base = strings.TrimSpace(content)
		return from
	}

	// Parse image and tag
	imageParts := strings.Split(content, ":")
	from.Base = strings.TrimSpace(imageParts[0])
	if len(imageParts) > 1 {
		from.Tag = strings.TrimSpace(imageParts[1])
	}

	// Check for dynamic parts (variables)
	from.BaseDynamic = strings.Contains(from.Base, "${") || strings.Contains(from.Base, "$")
	from.TagDynamic = strings.Contains(from.Tag, "${") || strings.Contains(from.Tag, "$")

	return from
}

// extractRun extracts information from a RUN directive
func extractRun(content string) *RunDetails {
	// Create a new RunDetails object
	details := &RunDetails{
		Packages: []string{},
	}

	// Extract the command part (after "RUN ")
	cmdStart := strings.Index(content, "RUN ")
	if cmdStart == -1 {
		return details
	}

	cmdContent := content[cmdStart+4:]

	// Parse the shell command
	cmd := shellparse2.NewShellCommand(cmdContent)
	details.Command = cmd

	// First, determine the distribution based on package manager commands
	var detectedPackageManager Manager
	for _, pm := range AllPackageManagers {
		pmStr := string(pm)

		// Check if this package manager is used in the command
		pmCmds := cmd.FindCommandsByPrefix(pmStr)
		if len(pmCmds) > 0 {
			// Found a package manager, get its associated distro
			info := PackageManagerInfoMap[pm]
			details.Distro = info.Distro
			detectedPackageManager = pm
			break
		}
	}

	// If no distro detected, set to unknown
	if details.Distro == "" {
		details.Distro = DistroUnknown
		return details
	}

	// Extract packages only for the detected package manager
	if detectedPackageManager != "" {
		pmStr := string(detectedPackageManager)
		info := PackageManagerInfoMap[detectedPackageManager]

		// Find install commands for this package manager
		installCmds := cmd.FindCommandsByPrefixAndSubcommand(pmStr, info.InstallKeyword)

		// Extract packages from each install command
		for _, installCmd := range installCmds {
			packages := shellparse2.ExtractPackagesFromInstallCommand(installCmd)
			details.Packages = append(details.Packages, packages...)
		}
	}

	// Deduplicate packages
	if len(details.Packages) > 0 {
		details.Packages = deduplicatePackages(details.Packages)
	}

	return details
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
