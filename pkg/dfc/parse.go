package dfc

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/chainguard-dev/dfc/internal/shellparse"
)

const (
	lineSep = "\n"
)

var (
	hasContinuationRegex       = regexp.MustCompile(`(?m).*\\\s*$`)
	extractRegex               = regexp.MustCompile(`(?s)^\s*(?P<directive>[A-Za-z]+)\s+(?P<command>.*)$`)
	extractRegexDirectiveIndex = extractRegex.SubexpIndex("directive")
	extractRegexCommandIndex   = extractRegex.SubexpIndex("command")
	fromRegex                  = regexp.MustCompile(`(?P<base>[^\s]+)\s+\bAS?\b\s+(?P<alias>[^\s]+).*`)
	fromRegexBaseIndex         = fromRegex.SubexpIndex("base")
	fromRegexAliasIndex        = fromRegex.SubexpIndex("alias")
	fromNoAliasRegex           = regexp.MustCompile(`(?P<base>[^\s]+).*`)
	fromNoAliasRegexBaseIndex  = fromRegex.SubexpIndex("base")
)

// TODO: use context (log some stuff)
func parse(_ context.Context, b []byte) (*Dockerfile, error) {
	d := &Dockerfile{Lines: []*DockerfileLine{}}
	stage := 0
	seenAliases := map[string]int{} // map of alises to prior stages
	prev := ""
	for _, raw := range strings.Split(string(b), lineSep) {
		// If we detect a line continuation, save the contents into the
		// prev var and skip this line
		if hasContinuationRegex.MatchString(raw) {
			if prev == "" {
				prev = raw
			} else {
				prev += lineSep + raw
			}
			continue
		}

		// If prev var is not empty, put it at the front of the line and reset it
		if prev != "" {
			raw = prev + lineSep + raw
			prev = ""
		}

		// Extract the directive (line type) and content/command
		directive, content := extractTopLevel(raw)

		// If we couldnt determine a line type, consider this whitespace or a comment
		// and append to the last line (unless a line doesnt exist yet)
		if directive == "" && len(d.Lines) > 0 {
			lastIdx := len(d.Lines) - 1
			extra := lineSep + raw
			d.Lines[lastIdx].Raw += extra
			if d.Lines[lastIdx].Directive != "" { // dont consider extra if the whole thing is extra
				d.Lines[lastIdx].Extra += extra
			}
			continue
		}

		dl := &DockerfileLine{
			Raw:       raw,
			Directive: directive,
			Content:   content,
		}

		// Extract more details specific to the directive
		switch directive {
		case DirectiveFrom:
			dl.From = extractFrom(content, &stage, &seenAliases)

		case DirectiveRun:
			dl.Run = extractRun(content)
		}

		dl.Stage = stage

		// Append the line
		d.Lines = append(d.Lines, dl)
	}

	return d, nil
}

func extractTopLevel(raw string) (string, string) {
	matches := extractRegex.FindStringSubmatch(raw)
	numMatches := len(matches)
	var lineType, content string
	if numMatches >= extractRegexDirectiveIndex {
		lineType = strings.ToUpper(matches[extractRegexDirectiveIndex])
	}
	if numMatches >= extractRegexCommandIndex {
		content = matches[extractRegexCommandIndex]
	}
	return lineType, content
}

func extractFrom(content string, stage *int, seenAliases *map[string]int) *FromDetails {
	// Increment the stage counter
	*stage++

	var base, tag, alias string
	var parent int
	matches := fromRegex.FindStringSubmatch(content)
	numMatches := len(matches)
	if numMatches >= fromRegexAliasIndex {
		alias = matches[fromRegexAliasIndex]

		// Modify seenAliases to include this entry
		m := *seenAliases
		m[alias] = *stage
		seenAliases = &m
	}
	if numMatches >= fromRegexBaseIndex {
		base = matches[fromRegexBaseIndex]
		m := *seenAliases
		if v, ok := m[base]; ok {
			parent = v
		}
	} else {
		// If we werent able to parse with the alias-style, try with non-alias (e.g. FROM node)
		matches = fromNoAliasRegex.FindStringSubmatch(content)
		numMatches = len(matches)
		if numMatches >= fromNoAliasRegexBaseIndex {
			base = matches[fromRegexBaseIndex]
		}
	}

	// Assuming we were able to detemrine the base,
	// attempt to extract the tag and parent
	if base != "" {
		if tmp := strings.SplitN(base, ":", 2); len(tmp) > 1 {
			base = tmp[0]
			tag = tmp[1]
		}
	}

	// If a $ is detected, mark it dynamic
	baseDynamic := strings.Contains(base, "$")
	tagDynamic := strings.Contains(tag, "$")

	return &FromDetails{
		Base:        base,
		Alias:       alias,
		Tag:         tag,
		Parent:      parent,
		BaseDynamic: baseDynamic,
		TagDynamic:  tagDynamic,
	}
}

func extractRun(content string) *RunDetails {
	details := &RunDetails{
		Packages: []string{},
	}
	cmd := shellparse.ParseShellLine(content)

	// Store the parsed command
	details.command = cmd

	// First, determine the distribution based on package manager commands
	// Check for each package manager and set the distribution accordingly
	for _, pm := range AllPackageManagers {
		pmCommands := cmd.GetCommandsByMultiExe([]string{string(pm)})
		if len(pmCommands) > 0 {
			info := PackageManagerInfoMap[pm]
			details.Distro = info.Distro
			break // Found a package manager, stop looking
		}
	}

	// If no distro detected, set to unknown
	if details.Distro == "" {
		details.Distro = DistroUnknown
	}

	// Get the package managers for this distribution
	packageManagers, exists := PackageManagerGroups[details.Distro]
	if exists {
		// Convert the package managers to strings for the GetCommandsByMultiExeAndSubcommand call
		pmStrings := make([]string, len(packageManagers))
		for i, pm := range packageManagers {
			pmStrings[i] = string(pm)

			// Get the install keyword for this package manager
			info := PackageManagerInfoMap[pm]

			// Extract packages from commands with this package manager and its install keyword
			installCommands := cmd.GetCommandsByMultiExeAndSubcommand(
				[]string{string(pm)}, info.InstallKeyword)
			details.Packages = append(details.Packages, extractPackagesFromCommands(installCommands)...)
		}

		slices.Sort(details.Packages)
		details.Packages = slices.Compact(details.Packages)
	}

	return details
}

// extractPackagesFromCommands parses package installation commands to extract just the package names,
// removing command-line flags like "-y"
func extractPackagesFromCommands(commands []string) []string {
	var packages []string

	// Create a set of all possible installation keywords
	installKeywords := make(map[string]bool)
	for _, pmInfo := range PackageManagerInfoMap {
		installKeywords[pmInfo.InstallKeyword] = true
	}

	for _, cmd := range commands {
		// Split the command into words
		parts := strings.Fields(cmd)

		// Find the index after the subcommand (install/add)
		cmdIndex := -1
		for i, part := range parts {
			if installKeywords[part] {
				cmdIndex = i
				break
			}
		}

		// Skip if we couldn't find the subcommand
		if cmdIndex == -1 || cmdIndex+1 >= len(parts) {
			continue
		}

		// Process each word after the subcommand
		for i := cmdIndex + 1; i < len(parts); i++ {
			// Skip flags (words starting with -)
			if strings.HasPrefix(parts[i], "-") {
				continue
			}

			// Add non-flags as packages
			packages = append(packages, parts[i])
		}
	}

	return packages
}
