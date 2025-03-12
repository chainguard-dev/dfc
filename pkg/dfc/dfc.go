package dfc

import (
	"context"
	"strings"

	"github.com/chainguard-dev/dfc/internal/shellparse"
)

const (
	// All supported distros
	DistroDebian  Distro = "debian"
	DistroFedora  Distro = "fedora"
	DistroAlpine  Distro = "alpine"
	DistroUbuntu  Distro = "ubuntu"
	DistroUnknown Distro = "unknown"

	// All supported package managers
	ManagerAptGet   Manager = "apt-get"
	ManagerApk      Manager = "apk"
	ManagerYum      Manager = "yum"
	ManagerDnf      Manager = "dnf"
	ManagerMicrodnf Manager = "microdnf"
	ManagerApt      Manager = "apt"

	// Directives we care about
	DirectiveFrom = "FROM"
	DirectiveRun  = "RUN"
	DirectiveUser = "USER"

	// Keywords for Dockerfile directives
	KeywordAs = "AS"

	// Command keywords
	KeywordAdd     = "add"
	KeywordInstall = "install"

	// Default values
	DefaultRegistryDomain = "cgr.dev"
	DefaultImageTag       = "latest-dev"
	DefaultUser           = "root"
	DummyCommand          = "true"

	// Package manager operation commands
	ApkInstallCommand = "add -U"
)

// PackageManagerInfo holds metadata about a package manager
type PackageManagerInfo struct {
	Distro         Distro // The associated distribution
	InstallKeyword string // The subcommand used for installation (e.g., "install" or "add")
}

// PackageManagerInfoMap maps each package manager to its metadata
var PackageManagerInfoMap = map[Manager]PackageManagerInfo{
	ManagerApk: {
		Distro:         DistroAlpine,
		InstallKeyword: KeywordAdd,
	},
	ManagerApt: {
		Distro:         DistroDebian,
		InstallKeyword: KeywordInstall,
	},
	ManagerAptGet: {
		Distro:         DistroDebian,
		InstallKeyword: KeywordInstall,
	},
	ManagerYum: {
		Distro:         DistroFedora,
		InstallKeyword: KeywordInstall,
	},
	ManagerDnf: {
		Distro:         DistroFedora,
		InstallKeyword: KeywordInstall,
	},
	ManagerMicrodnf: {
		Distro:         DistroFedora,
		InstallKeyword: KeywordInstall,
	},
}

// AllPackageManagers is a list of all supported package manager commands
// It's dynamically populated from the keys of PackageManagerInfoMap
var AllPackageManagers []Manager = func() []Manager {
	all := []Manager{}
	for pm := range PackageManagerInfoMap {
		all = append(all, pm)
	}
	return all
}()

// PackageManagerGroups groups package managers by distribution
// It's dynamically populated from PackageManagerInfoMap
var PackageManagerGroups = func() map[Distro][]Manager {
	groups := map[Distro][]Manager{}

	// Populate the groups based on the PackageManagerInfoMap
	for pm, info := range PackageManagerInfoMap {
		distro := info.Distro
		// Append this package manager to its distro group
		groups[distro] = append(groups[distro], pm)
	}

	return groups
}()

type (
	Dockerfile struct {
		Lines []*DockerfileLine `json:"lines"`
	}

	DockerfileLine struct {
		Raw       string       `json:"raw"`                 // Original content of the line
		Converted string       `json:"converted,omitempty"` // Converted line
		Directive string       `json:"directive,omitempty"` // Dockerfile directive such as FROM/RUN
		Extra     string       `json:"extra,omitempty"`     // Newlines and comments etc. that aren't part of the command itself
		Content   string       `json:"content,omitempty"`   // If valid directive, everything minus extra
		Stage     int          `json:"stage,omitempty"`     // stage of the Multistage Dockerfile build (1, 2, 3 ...)
		From      *FromDetails `json:"from,omitempty"`      // Details related to a FROM directive
		Run       *RunDetails  `json:"run,omitempty"`       // Details related to a RUN directive
	}

	// The information we could extract from a FROM line
	// FROM <base>:<tag> AS <alias>
	FromDetails struct {
		Base        string `json:"base,omitempty"`
		Tag         string `json:"tag,omitempty"`
		BaseDynamic bool   `json:"baseDynamic,omitempty"`
		TagDynamic  bool   `json:"tagDynamic,omitempty"`
		Alias       string `json:"alias,omitempty"`
		Parent      int    `json:"parent,omitempty"` // If the base is actually coming from a previous stage
	}

	// The information we could extract from a RUN line
	// RUN apt-get update && apt-get install -y <package1> <package2> # (<distro> = debian)
	RunDetails struct {
		Distro   Distro   `json:"distro,omitempty"` // Detected distro (if any)
		Packages []string `json:"packages"`         // Detected packages (if any)

		command *shellparse.ShellCommand // The parsed shell command (not exported to JSON)
	}

	// Enum types for each of our known line  distros and package managers
	Distro  string
	Manager string
)

func ParseDockerfile(ctx context.Context, b []byte) (*Dockerfile, error) {
	return parse(ctx, b)
}

func (d *Dockerfile) String() string {
	tmp := []string{}
	for _, line := range d.Lines {
		if line.Converted != "" {
			tmp = append(tmp, line.Converted)
		} else {
			tmp = append(tmp, line.Raw)
		}
	}
	return strings.Join(tmp, "\n")
}

type Options struct {
	Organization string
}

func (d *Dockerfile) Convert(ctx context.Context, opts *Options) error {
	return d.convert(ctx, opts)
}
