package dfc2

import (
	"context"
	"strings"

	"github.com/chainguard-dev/dfc/internal/shellparse2"
)

// Distro represents a Linux distribution
type Distro string

// Manager represents a package manager
type Manager string

// Supported distributions
const (
	DistroDebian  Distro = "debian"
	DistroFedora  Distro = "fedora"
	DistroAlpine  Distro = "alpine"
	DistroUbuntu  Distro = "ubuntu"
	DistroUnknown Distro = "unknown"
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
)

// PackageManagerInfo holds metadata about a package manager
type PackageManagerInfo struct {
	Distro         Distro
	InstallKeyword string
}

// PackageManagerInfoMap maps package managers to their metadata
var PackageManagerInfoMap = map[Manager]PackageManagerInfo{
	ManagerAptGet:   {Distro: DistroDebian, InstallKeyword: "install"},
	ManagerApk:      {Distro: DistroAlpine, InstallKeyword: "add"},
	ManagerYum:      {Distro: DistroFedora, InstallKeyword: "install"},
	ManagerDnf:      {Distro: DistroFedora, InstallKeyword: "install"},
	ManagerMicrodnf: {Distro: DistroFedora, InstallKeyword: "install"},
	ManagerApt:      {Distro: DistroDebian, InstallKeyword: "install"},
}

// PackageManagerGroups holds package managers grouped by distribution
var PackageManagerGroups map[Distro][]Manager

// AllPackageManagers holds a list of all supported package managers
var AllPackageManagers []Manager

// Initialize package manager groups and list
func init() {
	// Initialize groups
	PackageManagerGroups = make(map[Distro][]Manager)

	// Group package managers by distro
	for pm, info := range PackageManagerInfoMap {
		PackageManagerGroups[info.Distro] = append(PackageManagerGroups[info.Distro], pm)
	}

	// Make sure unknown distro is always included
	if _, exists := PackageManagerGroups[DistroUnknown]; !exists {
		PackageManagerGroups[DistroUnknown] = []Manager{}
	}

	// Initialize list of all package managers
	AllPackageManagers = make([]Manager, 0, len(PackageManagerInfoMap))
	for pm := range PackageManagerInfoMap {
		AllPackageManagers = append(AllPackageManagers, pm)
	}
}

// DockerfileLine represents a single line in a Dockerfile
type DockerfileLine struct {
	Raw         string       `json:"raw"`
	ExtraBefore string       `json:"extraBefore,omitempty"` // Comments and whitespace that appear before this line
	Directive   string       `json:"directive,omitempty"`
	Stage       int          `json:"stage,omitempty"`
	From        *FromDetails `json:"from,omitempty"`
	Run         *RunDetails  `json:"run,omitempty"`
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
	Command  *shellparse2.ShellCommand `json:"-"`
	Distro   Distro                    `json:",omitempty"`
	Packages []string                  `json:",omitempty"`
}

// Dockerfile represents a parsed Dockerfile
type Dockerfile struct {
	Lines        []*DockerfileLine `json:"lines"`
	StageAliases map[string]bool   `json:"stageAliases"` // Tracks stage aliases defined with AS
}

// String returns the Dockerfile content as a string
func (d *Dockerfile) String() string {
	var builder strings.Builder

	for i, line := range d.Lines {
		// Write any extra content that comes before this line
		if line.ExtraBefore != "" {
			// Write the extra content exactly as is - it should already contain the necessary newlines
			builder.WriteString(line.ExtraBefore)

			// If ExtraBefore doesn't end with a newline, add one
			if !strings.HasSuffix(line.ExtraBefore, "\n") {
				builder.WriteString("\n")
			}
		}

		// Skip empty directives (they're just comments or whitespace)
		if line.Directive == "" {
			continue
		}

		// Write the line itself
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
		StageAliases: make(map[string]bool),
	}

	// Copy stage aliases
	for alias, val := range d.StageAliases {
		newDf.StageAliases[alias] = val
	}

	// Copy lines
	for i, line := range d.Lines {
		// Create a deep copy of the line
		newLine := &DockerfileLine{
			Raw:         line.Raw,
			ExtraBefore: line.ExtraBefore,
			Directive:   line.Directive,
			Stage:       line.Stage,
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
			var newCommand *shellparse2.ShellCommand
			if line.Run.Command != nil {
				// Here we would ideally clone the command, but for simplicity let's reuse it
				// If shellparse2.ShellCommand had a Clone method, we would use it here
				newCommand = line.Run.Command
			}

			// Create new RunDetails
			newLine.Run = &RunDetails{
				Command:  newCommand,
				Distro:   line.Run.Distro,
				Packages: append([]string{}, line.Run.Packages...), // Copy slice
			}
		}

		newDf.Lines[i] = newLine
	}

	// Apply the conversion
	for _, line := range newDf.Lines {
		// Only process FROM and RUN directives
		switch line.Directive {
		case DirectiveFrom:
			convertFromDirective(line, opts, newDf.StageAliases)
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
