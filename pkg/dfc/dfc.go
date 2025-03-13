package dfc

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/chainguard-dev/dfc/internal/shellparse"
)

// Distro represents a Linux distribution
type Distro string

// Manager represents a package manager
type Manager string

// Supported distributions
const (
	DistroDebian Distro = "debian"
	DistroFedora Distro = "fedora"
	DistroAlpine Distro = "alpine"
)

// Supported package managers
const (
	ManagerAptGet   Manager = "apt-get"
	ManagerApk      Manager = "apk"
	ManagerYum      Manager = "yum"
	ManagerDnf      Manager = "dnf"
	ManagerMicrodnf Manager = "microdnf"
	ManagerApt      Manager = "apt"
)

// Install subcommands
const (
	SubcommandInstall = "install"
	SubcommandAdd     = "add"
)

// Dockerfile directives
const (
	DirectiveFrom = "FROM"
	DirectiveRun  = "RUN"
	DirectiveUser = "USER"
	KeywordAs     = "AS"
)

// Default values
const (
	DefaultRegistryDomain = "cgr.dev"
	DefaultBaseImage      = "alpine"
	DefaultImageTag       = "latest"
	DefaultUser           = "root"
	DefaultPackageManager = "apk"
	DefaultOrganization   = "ORGANIZATION"
	DefaultInstallCommand = "apk add -U"
)

// PackageManagerInfo holds metadata about a package manager
type PackageManagerInfo struct {
	Distro         Distro
	InstallKeyword string
}

// PackageManagerInfoMap maps package managers to their metadata
var PackageManagerInfoMap = map[Manager]PackageManagerInfo{
	ManagerAptGet: {Distro: DistroDebian, InstallKeyword: SubcommandInstall},
	ManagerApt:    {Distro: DistroDebian, InstallKeyword: SubcommandInstall},

	ManagerYum:      {Distro: DistroFedora, InstallKeyword: SubcommandInstall},
	ManagerDnf:      {Distro: DistroFedora, InstallKeyword: SubcommandInstall},
	ManagerMicrodnf: {Distro: DistroFedora, InstallKeyword: SubcommandInstall},

	ManagerApk: {Distro: DistroAlpine, InstallKeyword: SubcommandAdd},
}

// PackageManagerGroups holds package managers grouped by distribution
var PackageManagerGroups = map[Distro][]Manager{
	DistroDebian: {ManagerAptGet, ManagerApt},
	DistroFedora: {ManagerYum, ManagerDnf, ManagerMicrodnf},
	DistroAlpine: {ManagerApk},
}

// AllPackageManagers holds a list of all supported package managers
var AllPackageManagers = []Manager{
	ManagerAptGet, ManagerApt, ManagerYum, ManagerDnf, ManagerMicrodnf, ManagerApk,
}

// DockerfileLine represents a single line in a Dockerfile
type DockerfileLine struct {
	Raw       string       `json:"raw,omitempty"`
	Extra     string       `json:"extra,omitempty"` // Comments and whitespace that appear before this line
	Directive string       `json:"directive,omitempty"`
	Stage     int          `json:"stage,omitempty"`
	From      *FromDetails `json:"from,omitempty"`
	Run       *RunDetails  `json:"run,omitempty"`
}

// FromDetails holds details about a FROM directive
type FromDetails struct {
	Base        string `json:"base,omitempty"`
	Tag         string `json:"tag,omitempty"`
	Alias       string `json:"alias,omitempty"`
	Parent      int    `json:"parent,omitempty"`
	BaseDynamic bool   `json:"baseDynamic,omitempty"`
	TagDynamic  bool   `json:"tagDynamic,omitempty"`
}

// RunDetails holds details about a RUN directive
type RunDetails struct {
	Distro   Distro   `json:"distro,omitempty"`
	Packages []string `json:"packages,omitempty"`

	command *shellparse.ShellCommand `json:"-"`
}

// Dockerfile represents a parsed Dockerfile
type Dockerfile struct {
	Lines []*DockerfileLine `json:"lines"`

	stageAliases map[string]bool // Tracks stage aliases defined with AS
}

// String returns the Dockerfile content as a string
func (d *Dockerfile) String() string {
	var builder strings.Builder

	for i, line := range d.Lines {
		// Write any extra content that comes before this line
		if line.Extra != "" {
			extraContent := line.Extra

			// Special handling for comment blocks
			if strings.Contains(extraContent, "#") {
				// Check if this is the last line
				isLastLine := i == len(d.Lines)-1

				// For a trailing comment at the end of file with no directive
				if isLastLine && line.Raw == "" {
					// For trailing comments, preserve exactly as they were
					// The original content already has the right number of newlines at the end
					// Don't modify it at all - we want to preserve whether it ended with a newline or not

					// But if it ends with multiple newlines, normalize to at most one
					for strings.HasSuffix(extraContent, "\n\n") {
						extraContent = extraContent[:len(extraContent)-1]
					}
				} else {
					// For comments followed by directives, preserve original spacing
					// First, see if the content ends with a blank line (two consecutive newlines)
					hasBlankLineAfter := strings.HasSuffix(extraContent, "\n\n")

					// Normalize trailing newlines to get the comment content without excess newlines
					for strings.HasSuffix(extraContent, "\n") {
						extraContent = extraContent[:len(extraContent)-1]
					}

					// If original had a blank line after, add one blank line (two newlines)
					// Otherwise just add one newline to end the comment
					if hasBlankLineAfter {
						extraContent += "\n\n"
					} else {
						extraContent += "\n"
					}
				}
			}

			// Write the extra content
			builder.WriteString(extraContent)

			// If Extra doesn't end with a newline, add one before the directive
			// (only if we're not at the last line)
			if !strings.HasSuffix(extraContent, "\n") && i < len(d.Lines)-1 && line.Raw != "" {
				builder.WriteString("\n")
			}
		}

		// Write the line itself, regardless of whether it has a directive
		builder.WriteString(line.Raw)

		// Add a newline after each line except the last one
		if i < len(d.Lines)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// Convert applies the conversion to the Dockerfile and returns a new converted Dockerfile
func (d *Dockerfile) Convert(ctx context.Context, opts Options) *Dockerfile {
	// Create a deep copy of the Dockerfile
	newDf := &Dockerfile{
		Lines:        make([]*DockerfileLine, len(d.Lines)),
		stageAliases: make(map[string]bool),
	}

	// Copy stage aliases
	maps.Copy(newDf.stageAliases, d.stageAliases)

	// Copy lines
	for i, line := range d.Lines {
		// Create a deep copy of the line
		newLine := &DockerfileLine{
			Raw:       line.Raw,
			Extra:     line.Extra,
			Directive: line.Directive,
			Stage:     line.Stage,
		}

		// Copy From details if present
		if line.From != nil {
			newLine.From = &FromDetails{
				Base:        line.From.Base,
				Tag:         line.From.Tag,
				Alias:       line.From.Alias,
				Parent:      line.From.Parent,
				BaseDynamic: line.From.BaseDynamic,
				TagDynamic:  line.From.TagDynamic,
			}
		}

		// Copy Run details if present
		if line.Run != nil {
			// For Command, we need to copy or clone it from the original
			var newCommand *shellparse.ShellCommand
			if line.Run.command != nil {
				// Here we would ideally clone the command, but for simplicity let's reuse it
				// If shellparse2.ShellCommand had a Clone method, we would use it here
				newCommand = line.Run.command
			}

			// Create new RunDetails
			newLine.Run = &RunDetails{
				command:  newCommand,
				Distro:   line.Run.Distro,
				Packages: slices.Clone(line.Run.Packages), // Copy slice
			}
		}

		newDf.Lines[i] = newLine
	}

	// Apply the conversion
	for _, line := range newDf.Lines {
		// Only process FROM and RUN directives
		switch line.Directive {
		case DirectiveFrom:
			convertFromDirective(line, opts, newDf.stageAliases)
		case DirectiveRun:
			convertRunDirective(line, opts)
		}
	}

	return newDf
}

// Options represents conversion options
type Options struct {
	Organization string
	PackageMap   map[string]string
}
