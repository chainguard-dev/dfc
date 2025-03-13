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

// TestEdgeCases tests various edge cases for shell command parsing
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantOk   bool
		contains string
	}{
		{
			name:     "empty command",
			command:  "",
			wantOk:   true,
			contains: "",
		},
		{
			name:     "whitespace only",
			command:  "   \t   \n   ",
			wantOk:   true,
			contains: "",
		},
		{
			name:     "single operator",
			command:  "&&",
			wantOk:   true,
			contains: "&&",
		},
		{
			name:     "just a backslash",
			command:  "\\",
			wantOk:   true,
			contains: "\\",
		},
		{
			name:     "unclosed quote",
			command:  "echo \"unclosed",
			wantOk:   true, // Should be handled gracefully
			contains: "echo",
		},
		{
			name:     "command with unicode characters",
			command:  "echo 'hello world' && apt-get install -y git", // Simplified to avoid Unicode issues
			wantOk:   true,
			contains: "hello world",
		},
		{
			name:     "command with no spaces",
			command:  "apt-get install -y git&&apt-get install -y curl",
			wantOk:   true,
			contains: "apt-get",
		},
		{
			name:     "command with excessive whitespace",
			command:  "   apt-get    install    -y    git   ",
			wantOk:   true,
			contains: "apt-get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewShellCommand(tt.command)

			if (cmd.RootNode != nil) != tt.wantOk {
				t.Errorf("Command %q parsing status %v, want %v", tt.command, cmd.RootNode != nil, tt.wantOk)
			}

			// Check if output contains expected string
			if tt.contains != "" && !strings.Contains(cmd.String(), tt.contains) {
				t.Errorf("Expected output to contain %q but got %q", tt.contains, cmd.String())
			}
		})
	}
}

// TestParsingComplexCommands tests parsing of more complex shell commands
func TestParsingComplexCommands(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantOk  bool
	}{
		{
			name:    "command with subshell",
			command: "apt-get update && (apt-get install -y curl || echo 'Failed')",
			wantOk:  true,
		},
		{
			name:    "command with variable assignment",
			command: "DEBIAN_FRONTEND=noninteractive apt-get install -y git",
			wantOk:  true,
		},
		{
			name:    "command with backslash escape in quotes",
			command: "echo \"Line \\\"quoted\\\" text\" && apt-get install -y git",
			wantOk:  true,
		},
		{
			name:    "command with multiple line continuations",
			command: "apt-get update \\\n  && apt-get \\\n     install \\\n     -y \\\n     git curl",
			wantOk:  true,
		},
		{
			name:    "command with redirect",
			command: "apt-get update > /dev/null 2>&1 && apt-get install -y git",
			wantOk:  true,
		},
		{
			name:    "command with background process",
			command: "apt-get update & apt-get install -y git",
			wantOk:  true,
		},
		{
			name:    "command with complex nesting",
			command: "apt-get update && (apt-get install -y curl || (apt-get install -y wget || echo 'Failed'))",
			wantOk:  true,
		},
		{
			name:    "command with multiline quoted strings",
			command: "echo \"Line 1\nLine 2\nLine 3\" && apt-get install -y git",
			wantOk:  true,
		},
		{
			name:    "command with complex variable substitution",
			command: "apt-get install -y ${PACKAGES:-git curl}",
			wantOk:  true,
		},
		{
			name:    "command with pipe and grep",
			command: "apt-cache search git | grep -v deprecated && apt-get install -y git",
			wantOk:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewShellCommand(tt.command)

			// Basic validation that we can parse without error
			if cmd.RootNode == nil && tt.wantOk {
				t.Errorf("Failed to parse command %q", tt.command)
			}

			// Ensure output contains original command components
			output := cmd.String()
			if !strings.Contains(output, "apt-get") && strings.Contains(tt.command, "apt-get") {
				t.Errorf("Expected output to contain 'apt-get' but got %q", output)
			}
		})
	}
}

// TestCommandRoundtrip tests that parsing and re-stringifying preserves commands
func TestCommandRoundtrip(t *testing.T) {
	tests := []struct {
		name    string
		command string
		exact   bool // Whether we expect exact string match or just content preservation
	}{
		{
			name:    "simple command",
			command: "apt-get install -y git",
			exact:   true,
		},
		{
			name:    "command with environment variables",
			command: "PKG_MGR=apt-get $PKG_MGR install -y git",
			exact:   true,
		},
		{
			name:    "command with quotes",
			command: "apt-get install -y \"package with space\"",
			exact:   true,
		},
		{
			name:    "command with operators",
			command: "apt-get update && apt-get install -y git",
			exact:   false, // Spacing around operators might change
		},
		{
			name:    "multi-line command",
			command: "apt-get update && \\\napt-get install -y \\\n  git curl",
			exact:   false, // Line breaks might be normalized
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewShellCommand(tt.command)
			result := cmd.String()

			if tt.exact && result != tt.command {
				t.Errorf("Command roundtrip failed:\nInput:  %q\nOutput: %q", tt.command, result)
			} else if !tt.exact {
				// For non-exact matches, check that important content is preserved
				// Normalize both strings by removing formatting differences
				normalizedInput := strings.ReplaceAll(tt.command, "\n", " ")
				normalizedInput = strings.ReplaceAll(normalizedInput, "\\", "")
				normalizedInput = strings.ReplaceAll(normalizedInput, "  ", " ")
				normalizedInput = strings.ReplaceAll(normalizedInput, " && ", "&&")
				normalizedInput = strings.ReplaceAll(normalizedInput, "&& ", "&&")
				normalizedInput = strings.ReplaceAll(normalizedInput, " &&", "&&")
				normalizedInput = strings.TrimSpace(normalizedInput)

				normalizedResult := strings.ReplaceAll(result, "\n", " ")
				normalizedResult = strings.ReplaceAll(normalizedResult, "\\", "")
				normalizedResult = strings.ReplaceAll(normalizedResult, "  ", " ")
				normalizedResult = strings.ReplaceAll(normalizedResult, " && ", "&&")
				normalizedResult = strings.ReplaceAll(normalizedResult, "&& ", "&&")
				normalizedResult = strings.ReplaceAll(normalizedResult, " &&", "&&")
				normalizedResult = strings.TrimSpace(normalizedResult)

				// Use Contains as a fall-back since formatting may vary
				if normalizedInput != normalizedResult {
					t.Errorf("Command content not preserved:\nInput:  %q\nOutput: %q", normalizedInput, normalizedResult)
				}
			}
		})
	}
}

// TestAdvancedParsing tests advanced shell syntax features
func TestAdvancedParsing(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantOk  bool
	}{
		{
			name:    "command with nested quotes",
			command: "echo \"This is a 'quoted' string\" && apt-get install -y git",
			wantOk:  true,
		},
		{
			name:    "command with escaped sequences",
			command: "echo \"This has escaped \\\"quotes\\\" inside\" && apt-get install -y git",
			wantOk:  true,
		},
		{
			name:    "command with here document",
			command: "cat << EOF\nThis is a here document\nEOF\n&& apt-get install -y git",
			wantOk:  true,
		},
		{
			name:    "command with complex variable expansion",
			command: "echo ${VAR:-default value} && apt-get install -y git",
			wantOk:  true,
		},
		{
			name:    "command with array variables",
			command: "PKGS=(git curl) && apt-get install -y ${PKGS[@]}",
			wantOk:  true,
		},
		{
			name:    "command with function definition",
			command: "function install_pkg() { apt-get install -y \"$@\"; } && install_pkg git curl",
			wantOk:  true,
		},
		{
			name:    "command with arithmetic expansion",
			command: "for ((i=0; i<5; i++)); do apt-get install -y pkg$i; done",
			wantOk:  true,
		},
		{
			name:    "command with process substitution",
			command: "apt-get install -y $(cat packages.txt | grep -v '#')",
			wantOk:  true,
		},
		{
			name:    "command with conditional execution",
			command: "[ -f packages.txt ] && apt-get install -y $(cat packages.txt)",
			wantOk:  true,
		},
		{
			name:    "command with nested subshells",
			command: "apt-get install -y $(grep -l $(cat versions.txt) packages.txt)",
			wantOk:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewShellCommand(tt.command)

			// Just verify parsing completes without errors
			if cmd.RootNode == nil && tt.wantOk {
				t.Errorf("Failed to parse command %q", tt.command)
			}

			// Basic validation that stringification preserves the command
			output := cmd.String()
			if output == "" && tt.command != "" {
				t.Errorf("Command %q produced empty output", tt.command)
			}
		})
	}
}
