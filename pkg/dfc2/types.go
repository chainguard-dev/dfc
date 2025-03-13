package dfc2

import (
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
	stageAliases map[string]bool   // Tracks stage aliases defined with AS
}

// Options represents conversion options
type Options struct {
	Organization string
	PackageMap   map[string]string
}
