package shellparse

import (
	"strings"
	"testing"
)

func TestShellCommandParsing(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		wantParts    []string
		wantCmdCount int
	}{
		{
			name:         "simple command",
			command:      "apt-get install -y nginx",
			wantParts:    []string{"apt-get", "install", "-y", "nginx"},
			wantCmdCount: 1,
		},
		{
			name:         "multiple commands",
			command:      "apt-get update && apt-get install -y nginx",
			wantParts:    []string{"apt-get", "install", "-y", "nginx"},
			wantCmdCount: 2,
		},
		{
			name:         "command with line continuation",
			command:      "apt-get update && \\\n    apt-get install -y \\\n    nginx curl",
			wantParts:    []string{"apt-get", "install", "-y", "nginx", "curl"},
			wantCmdCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewShellCommand(tt.command)

			// Check command parsing
			if cmd.RootNode == nil {
				t.Errorf("Expected RootNode to be non-nil")
			}

			// Check if we can find commands
			installCmds := cmd.FindCommandsByPrefixAndSubcommand("apt-get", "install")
			if len(installCmds) == 0 {
				t.Errorf("Expected to find 'apt-get install' command")
			}

			// Verify command parts
			for _, part := range tt.wantParts {
				if !strings.Contains(cmd.String(), part) {
					t.Errorf("Expected output to contain %q, but got %q", part, cmd.String())
				}
			}
		})
	}
}

func TestFindAndReplace(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		findPrefix     string
		findSubcmd     string
		replacement    string
		expectedCount  int
		wantContain    string
		wantNotContain string
	}{
		{
			name:           "replace apt-get install",
			command:        "apt-get update && apt-get install -y git",
			findPrefix:     "apt-get",
			findSubcmd:     "install",
			replacement:    "apk add -U git",
			expectedCount:  1,
			wantContain:    "apk add -U git",
			wantNotContain: "apt-get install",
		},
		{
			name:           "multi-line command",
			command:        "apt-get update && \\\napt-get install -y \\\n  git curl",
			findPrefix:     "apt-get",
			findSubcmd:     "install",
			replacement:    "apk add -U git curl",
			expectedCount:  1,
			wantContain:    "apk add -U git curl",
			wantNotContain: "apt-get install",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewShellCommand(tt.command)

			// Find command
			installCmds := cmd.FindCommandsByPrefixAndSubcommand(tt.findPrefix, tt.findSubcmd)

			// Check that we found the expected commands
			if len(installCmds) != tt.expectedCount {
				t.Errorf("Expected to find %d commands, found %d", tt.expectedCount, len(installCmds))
			}

			// Replace first command
			if len(installCmds) > 0 {
				cmd.ReplaceCommand(installCmds[0], tt.replacement)
			}

			// Check result
			result := cmd.String()
			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("Expected result to contain %q, but got %q", tt.wantContain, result)
			}

			if strings.Contains(result, tt.wantNotContain) {
				t.Errorf("Expected result not to contain %q, but it does: %q", tt.wantNotContain, result)
			}
		})
	}
}

func TestExtractPackages(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantPkgs []string
	}{
		{
			name:     "simple packages",
			command:  "apt-get install -y git curl",
			wantPkgs: []string{"git", "curl"},
		},
		{
			name:     "with options and variables",
			command:  "apt-get install -y --no-install-recommends $BUILDARG git curl",
			wantPkgs: []string{"git", "curl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewShellCommand(tt.command)
			installCmds := cmd.FindCommandsByPrefixAndSubcommand("apt-get", "install")

			if len(installCmds) == 0 {
				t.Errorf("Expected to find 'apt-get install' command")
				return
			}

			packages := ExtractPackagesFromInstallCommand(installCmds[0])

			// Check that we got the expected packages
			for _, pkg := range tt.wantPkgs {
				found := false
				for _, extractedPkg := range packages {
					if extractedPkg == pkg {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("Expected to find package %q, but it was not extracted. Got: %v", pkg, packages)
				}
			}
		})
	}
}
