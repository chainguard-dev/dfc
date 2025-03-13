package dfc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

/*
func TestConvertDockerfile(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		opts           Options
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "simple debian to alpine conversion",
			input: `FROM debian:11
RUN apt-get update && apt-get install -y nginx curl
CMD ["nginx", "-g", "daemon off;"]`,
			opts: Options{
				PackageMap: map[Distro]map[string][]string{
					DistroDebian: {
						"nginx": {"nginx"},
						"curl":  {"curl"},
					},
					DistroAlpine: {},
					DistroFedora: {},
				},
			},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/debian:latest-dev",
				"USER root",
				"apk add -U curl nginx",
			},
			wantNotContain: []string{
				"apt-get",
				"debian:11",
			},
		},
		{
			name: "multi-stage build",
			input: `FROM golang:1.18 AS builder
WORKDIR /app
COPY . .
RUN go build -o app

FROM debian:11
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=builder /app/app /app
CMD ["/app"]`,
			opts: Options{
				PackageMap: map[Distro]map[string][]string{
					DistroDebian: {
						"ca-certificates": {"ca-certificates"},
					},
					DistroAlpine: {},
					DistroFedora: {},
				},
			},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/golang:latest-dev AS builder",
				"USER root",
				"FROM cgr.dev/ORGANIZATION/debian:latest-dev",
				"USER root",
				"apk add -U ca-certificates",
			},
			wantNotContain: []string{
				"apt-get",
				"debian:11",
			},
		},
		{
			name: "custom organization",
			input: `FROM ubuntu:20.04
RUN apt-get update && apt-get install -y python3 python3-pip
CMD ["python3", "-m", "http.server"]`,
			opts: Options{
				Organization: "myorg",
				PackageMap: map[Distro]map[string][]string{
					DistroDebian: {
						"python3":     {"python3"},
						"python3-pip": {"py3-pip"},
					},
					DistroAlpine: {},
					DistroFedora: {},
				},
			},
			wantContains: []string{
				"FROM cgr.dev/myorg/ubuntu:latest-dev",
				"USER root",
				"apk add -U py3-pip python3",
			},
			wantNotContain: []string{
				"apt-get",
				"ubuntu:20.04",
			},
		},
		{
			name: "preserves formatting and comments",
			input: `FROM debian:11
# Install dependencies
RUN apt-get update && \
    apt-get install -y \
      nginx \
      curl \
      vim
# Run the application
CMD ["nginx", "-g", "daemon off;"]`,
			opts: Options{
				PackageMap: map[Distro]map[string][]string{
					DistroDebian: {
						"nginx": {"nginx"},
						"curl":  {"curl"},
						"vim":   {"vim"},
					},
					DistroAlpine: {},
					DistroFedora: {},
				},
			},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/debian:latest-dev",
				"USER root",
				"# Install dependencies",
				"# Run the application",
			},
		},
		{
			name: "preserves comment spacing without blank line",
			input: `FROM debian:11
# Install dependencies
RUN apt-get update && \
    apt-get install -y \
      nginx \
      curl \
      vim
# Run the application
CMD ["nginx", "-g", "daemon off;"]`,
			opts: Options{
				PackageMap: map[Distro]map[string][]string{
					DistroDebian: {
						"nginx": {"nginx"},
						"curl":  {"curl"},
						"vim":   {"vim"},
					},
					DistroAlpine: {},
					DistroFedora: {},
				},
			},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/debian:latest-dev",
				"USER root",
				"# Install dependencies",
				"# Run the application",
			},
		},
		{
			name: "preserves comment spacing with blank line",
			input: `FROM node:20.15.0 AS base

# my comment
ARG ABC`,
			opts: Options{},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/node:latest-dev AS base",
				"USER root",
				"# my comment\nARG ABC",
			},
			wantNotContain: []string{
				"# my comment\n\nARG ABC",
			},
		},
		{
			name: "preserves comment spacing with blank line",
			input: `FROM node:20.15.0 AS base

# comment with blank line after

ARG ABC`,
			opts: Options{},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/node:latest-dev AS base",
				"USER root",
				"# comment with blank line after\n\nARG ABC",
			},
		},
		{
			name: "preserves mixed comment spacing",
			input: `FROM node:20.15.0 AS base

# comment with blank line after

ARG ABC

# comment without blank line
CMD ["echo", "hello"]`,
			opts: Options{},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/node:latest-dev AS base",
				"USER root",
				"# comment with blank line after\n\nARG ABC",
				"# comment without blank line\nCMD",
			},
			wantNotContain: []string{
				"# comment without blank line\n\nCMD",
			},
		},
		{
			name: "preserves multi-line comments",
			input: `FROM ubuntu:22.04

# This is a comment
# This is a second line of the comment
# This is a third line of the comment

COPY . .

CMD ["echo", "hello"]`,
			opts: Options{},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/ubuntu:latest-dev",
				"USER root",
				"# This is a comment\n# This is a second line of the comment\n# This is a third line of the comment\n\nCOPY",
			},
		},
		{
			name: "preserves trailing comments exact ending",
			input: `FROM ubuntu:22.04

RUN apt-get update

# This is a trailing comment`,
			opts: Options{},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/ubuntu:latest-dev",
				"USER root",
				"RUN true",
				"# This is a trailing comment",
			},
			wantNotContain: []string{
				"# This is a trailing comment\n\n",
			},
		},
		{
			name: "preserves trailing comments with newline",
			input: `FROM ubuntu:22.04

RUN apt-get update

# This is a trailing comment
`,
			opts: Options{},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/ubuntu:latest-dev",
				"USER root",
				"RUN true",
				"# This is a trailing comment\n",
			},
			wantNotContain: []string{
				"# This is a trailing comment\n\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := ConvertDockerfile(ctx, []byte(tt.input), tt.opts)
			if err != nil {
				t.Fatalf("ConvertDockerfile() error = %v", err)
			}

			gotStr := string(got)

			// Check that expected contents are present
			for _, want := range tt.wantContains {
				if !strings.Contains(gotStr, want) {
					t.Errorf("ConvertDockerfile() output does not contain %q, output:\n%s", want, gotStr)
				}
			}

			// Check that unwanted contents are not present
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(gotStr, notWant) {
					t.Errorf("ConvertDockerfile() output contains %q, but it shouldn't, output:\n%s", notWant, gotStr)
				}
			}
		})
	}
}

func TestParseDockerfile(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  string
		wantBase  string
		wantTag   string
		wantAlias string
	}{
		{
			name:     "simple from directive",
			input:    "FROM debian:11",
			wantType: DirectiveFrom,
			wantBase: "debian",
			wantTag:  "11",
		},
		{
			name:      "from with as",
			input:     "FROM golang:1.18 AS builder",
			wantType:  DirectiveFrom,
			wantBase:  "golang",
			wantTag:   "1.18",
			wantAlias: "builder",
		},
		{
			name:      "dynamic tag with as",
			input:     "FROM abc:$woo as stage",
			wantType:  DirectiveFrom,
			wantBase:  "abc",
			wantTag:   "$woo",
			wantAlias: "stage",
		},
		{
			name:      "mixed-case as keyword",
			input:     "FROM golang:1.18 aS builder",
			wantType:  DirectiveFrom,
			wantBase:  "golang",
			wantTag:   "1.18",
			wantAlias: "builder",
		},
		{
			name:      "dynamic tag with mixed-case as",
			input:     "FROM abc:$woo As stage",
			wantType:  DirectiveFrom,
			wantBase:  "abc",
			wantTag:   "$woo",
			wantAlias: "stage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			df, err := ParseDockerfile(ctx, []byte(tt.input))
			if err != nil {
				t.Fatalf("ParseDockerfile() error = %v", err)
			}

			if len(df.Lines) == 0 {
				t.Fatalf("ParseDockerfile() parsed no lines")
			}

			line := df.Lines[0]
			t.Logf("Parsed line: %+v", line)
			if line.Directive != tt.wantType {
				t.Errorf("ParseDockerfile() directive = %v, want %v", line.Directive, tt.wantType)
			}

			if line.Directive == DirectiveFrom && line.From != nil {
				if line.From.Base != tt.wantBase {
					t.Errorf("ParseDockerfile() FROM base = %v, want %v", line.From.Base, tt.wantBase)
				}
				if tt.wantTag != "" && line.From.Tag != tt.wantTag {
					t.Errorf("ParseDockerfile() FROM tag = %v, want %v", line.From.Tag, tt.wantTag)
				}
				if tt.wantAlias != "" && line.From.Alias != tt.wantAlias {
					t.Errorf("ParseDockerfile() FROM alias = %v, want %v", line.From.Alias, tt.wantAlias)
				}
			}
		})
	}
}
*/

func TestConvertFromDirective(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		opts       Options
		wantBase   string
		wantKeep   bool // If true, expect the base to remain unchanged
		wantReturn bool // Expected return value from the function
	}{
		{
			name:       "simple from directive",
			input:      "FROM debian:11",
			opts:       Options{},
			wantBase:   "cgr.dev/ORGANIZATION/debian",
			wantKeep:   false,
			wantReturn: true,
		},
		{
			name:       "from with as",
			input:      "FROM golang:1.18 AS builder",
			opts:       Options{},
			wantBase:   "cgr.dev/ORGANIZATION/golang",
			wantKeep:   false,
			wantReturn: true,
		},
		{
			name:       "custom organization",
			input:      "FROM ubuntu:20.04",
			opts:       Options{Organization: "myorg"},
			wantBase:   "cgr.dev/myorg/ubuntu",
			wantKeep:   false,
			wantReturn: true,
		},
		{
			name:       "with repository prefix",
			input:      "FROM someupstream/somebase:1.0",
			opts:       Options{},
			wantBase:   "cgr.dev/ORGANIZATION/somebase",
			wantKeep:   false,
			wantReturn: true,
		},
		{
			name:       "stage reference",
			input:      "FROM builder",
			opts:       Options{},
			wantBase:   "builder",
			wantKeep:   true,
			wantReturn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a stageAliases map for the "stage reference" test
			stageAliases := make(map[string]bool)
			if tt.name == "stage reference" {
				stageAliases["builder"] = true
			}

			df, err := ParseDockerfile(context.Background(), []byte(tt.input))
			if err != nil {
				t.Fatalf("Failed to parse dockerfile: %v", err)
			}

			line := df.Lines[0]
			t.Logf("Before conversion: %+v", line)

			// Apply conversion and capture the return value
			result := convertFromDirective(line, tt.opts, stageAliases)

			t.Logf("After conversion: %+v", line)
			t.Logf("Raw line: %s", line.Raw)

			// Check the return value
			if result != tt.wantReturn {
				t.Errorf("convertFromDirective() returned %v, want %v", result, tt.wantReturn)
			}

			if line.Directive == DirectiveFrom && line.From != nil {
				if tt.wantKeep {
					// Should not have changed
					if !strings.Contains(line.Raw, tt.wantBase) {
						t.Errorf("convertFromDirective() changed a line that should be kept, got %v, want to contain %v",
							line.Raw, tt.wantBase)
					}
				} else {
					// Should have changed
					if !strings.Contains(line.Raw, tt.wantBase) {
						t.Errorf("convertFromDirective() did not update raw line correctly, got %v, want to contain %v",
							line.Raw, tt.wantBase)
					}
				}
			}
		})
	}
}

// TestDockerfileString tests the String() method directly to ensure proper formatting preservation
func TestDockerfileString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "preserves comment spacing without blank line",
			input: `FROM node:20.15.0
# my comment
ARG ABC`,
			expected: `FROM node:20.15.0
# my comment
ARG ABC`,
		},
		{
			name: "preserves comment spacing with blank line",
			input: `FROM node:20.15.0

# comment with blank line after

ARG ABC`,
			expected: `FROM node:20.15.0

# comment with blank line after

ARG ABC`,
		},
		{
			name: "preserves multi-line comments",
			input: `FROM node:20.15.0

# This is a comment
# This is a second line of the comment
# This is a third line of the comment

COPY . .`,
			expected: `FROM node:20.15.0

# This is a comment
# This is a second line of the comment
# This is a third line of the comment

COPY . .`,
		},
		{
			name: "preserves trailing comments without newline",
			input: `FROM node:20.15.0
# This is a trailing comment`,
			expected: `FROM node:20.15.0
# This is a trailing comment`,
		},
		{
			name: "preserves trailing comments with newline",
			input: `FROM node:20.15.0
# This is a trailing comment
`,
			expected: `FROM node:20.15.0
# This is a trailing comment
`,
		},
		{
			name: "handles blank lines between directives",
			input: `FROM node:20.15.0


WORKDIR /app


CMD ["node", "app.js"]`,
			expected: `FROM node:20.15.0



WORKDIR /app



CMD ["node", "app.js"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			dockerfile, err := ParseDockerfile(ctx, []byte(tt.input))
			if err != nil {
				t.Fatalf("ParseDockerfile() error = %v", err)
			}

			got := dockerfile.String()
			if got != tt.expected {
				t.Errorf("String() output does not match expected.\nGot:\n%s\nExpected:\n%s", got, tt.expected)
			}
		})
	}
}

func TestImageMapping(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		imageMap ImageMap
		wantBase string
		opts     Options
	}{
		{
			name:  "exact match in image map",
			input: "FROM nginx:1.19",
			imageMap: ImageMap{
				Mappings: []ImageMapping{
					{Source: "nginx:1.19", Target: "nginx"},
				},
			},
			wantBase: "cgr.dev/ORGANIZATION/nginx",
			opts:     Options{},
		},
		{
			name:  "distroless best guess match",
			input: "FROM gcr.io/distroless/nodejs20-debian12",
			imageMap: ImageMap{
				Mappings: []ImageMapping{
					{Source: "node", Target: "node"},
					{Source: "python", Target: "python"},
					{Source: "golang", Target: "golang"},
				},
			},
			wantBase: "cgr.dev/ORGANIZATION/node",
			opts:     Options{},
		},
		{
			name:  "fallback to basename with no match",
			input: "FROM somevendor/somecustomimage:1.0",
			imageMap: ImageMap{
				Mappings: []ImageMapping{
					{Source: "node", Target: "node"},
					{Source: "python", Target: "python"},
				},
			},
			wantBase: "cgr.dev/ORGANIZATION/somecustomimage",
			opts:     Options{},
		},
		{
			name:  "custom organization with image map",
			input: "FROM nginx:1.19",
			imageMap: ImageMap{
				Mappings: []ImageMapping{
					{Source: "nginx:1.19", Target: "nginx"},
				},
			},
			wantBase: "cgr.dev/myorg/nginx",
			opts:     Options{Organization: "myorg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the ImageMap in options
			tt.opts.ImageMap = tt.imageMap

			df, err := ParseDockerfile(context.Background(), []byte(tt.input))
			if err != nil {
				t.Fatalf("Failed to parse dockerfile: %v", err)
			}

			line := df.Lines[0]
			t.Logf("Before conversion: %+v", line)

			// Apply conversion
			convertFromDirective(line, tt.opts, nil)

			t.Logf("After conversion: %+v", line)
			t.Logf("Raw line: %s", line.Raw)

			if !strings.Contains(line.Raw, tt.wantBase) {
				t.Errorf("convertFromDirective() did not update raw line correctly, got %v, want to contain %v",
					line.Raw, tt.wantBase)
			}
		})
	}
}

// TestFileBasedCases dynamically tests the conversion using pairs of .before.Dockerfile and .after.Dockerfile files
func TestFileBasedCases(t *testing.T) {
	// Define the test directory
	testDir := "testdata"

	// Create the test directory if it doesn't exist
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Get all files in the test directory
	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read test directory: %v", err)
	}

	// Map to hold the before/after pairs
	type testPair struct {
		before  string
		after   string
		options string // path to options YAML file, if it exists
	}
	testCases := make(map[string]testPair)

	// Find all .before.Dockerfile files and their corresponding .after.Dockerfile files
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".before.Dockerfile") {
			baseName := strings.TrimSuffix(name, ".before.Dockerfile")
			afterFile := baseName + ".after.Dockerfile"
			optionsFile := baseName + ".options.yaml"

			// Check if the corresponding .after file exists
			afterPath := filepath.Join(testDir, afterFile)
			if _, err := os.Stat(afterPath); err == nil {
				pair := testPair{
					before: filepath.Join(testDir, name),
					after:  afterPath,
				}

				// Check if options file exists
				optionsPath := filepath.Join(testDir, optionsFile)
				if _, err := os.Stat(optionsPath); err == nil {
					pair.options = optionsPath
				}

				testCases[baseName] = pair
			} else {
				t.Logf("Warning: No matching .after.Dockerfile found for %s", name)
			}
		}
	}

	// If no test cases were found, create a sample test case
	if len(testCases) == 0 {
		t.Log("No test cases found, creating sample test case")
		beforeContent := `FROM debian:11
RUN apt-get update && apt-get install -y nginx curl
CMD ["nginx", "-g", "daemon off;"]`
		afterContent := `FROM cgr.dev/ORGANIZATION/debian:latest-dev
USER root
RUN apk add -U curl nginx
CMD ["nginx", "-g", "daemon off;"]`

		beforePath := filepath.Join(testDir, "sample.before.Dockerfile")
		afterPath := filepath.Join(testDir, "sample.after.Dockerfile")

		if err := os.WriteFile(beforePath, []byte(beforeContent), 0644); err != nil {
			t.Fatalf("Failed to write sample before file: %v", err)
		}
		if err := os.WriteFile(afterPath, []byte(afterContent), 0644); err != nil {
			t.Fatalf("Failed to write sample after file: %v", err)
		}

		testCases["sample"] = testPair{
			before: beforePath,
			after:  afterPath,
		}

		t.Logf("Created sample test case in %s", testDir)
	}

	// Run each test case
	for name, pair := range testCases {
		t.Run(name, func(t *testing.T) {
			// Read the before file
			beforeBytes, err := os.ReadFile(pair.before)
			if err != nil {
				t.Fatalf("Failed to read before file: %v", err)
			}

			// Read the after file (expected output)
			expectedBytes, err := os.ReadFile(pair.after)
			if err != nil {
				t.Fatalf("Failed to read after file: %v", err)
			}
			expected := string(expectedBytes)

			// Default options for conversion
			opts := Options{
				Organization: DefaultOrganization,
				PackageMap: map[Distro]map[string][]string{
					DistroDebian: {
						"nginx":           {"nginx"},
						"curl":            {"curl"},
						"ca-certificates": {"ca-certificates"},
					},
					DistroAlpine: {
						"python3": {"python3"},
						"py3-pip": {"py3-pip"},
					},
					DistroFedora: {},
				},
			}

			// If options file exists, read and apply it
			if pair.options != "" {
				t.Logf("Using options from %s", pair.options)
				optionsBytes, err := os.ReadFile(pair.options)
				if err != nil {
					t.Fatalf("Failed to read options file: %v", err)
				}

				// Define a struct to hold the options
				type optionsYAML struct {
					Organization string                         `yaml:"organization"`
					ImageMap     map[string]string              `yaml:"imageMap"`
					PackageMap   map[string]map[string][]string `yaml:"packageMap"`
				}

				var yamlOpts optionsYAML
				if err := yaml.Unmarshal(optionsBytes, &yamlOpts); err != nil {
					t.Fatalf("Failed to parse options YAML: %v", err)
				}

				// Apply the options from the YAML file
				if yamlOpts.Organization != "" {
					opts.Organization = yamlOpts.Organization
				}

				// Copy the image mappings
				if yamlOpts.ImageMap != nil {
					for k, v := range yamlOpts.ImageMap {
						opts.ImageMap.Mappings = append(opts.ImageMap.Mappings, ImageMapping{Source: k, Target: v})
					}
				}

				if yamlOpts.PackageMap != nil {
					// Convert the string map keys to Distro enum
					for distroStr, pkgMap := range yamlOpts.PackageMap {
						var distro Distro
						switch strings.ToLower(distroStr) {
						case "debian":
							distro = DistroDebian
						case "alpine":
							distro = DistroAlpine
						case "fedora":
							distro = DistroFedora
						default:
							t.Fatalf("Unknown distro: %s", distroStr)
						}

						// If the distro map doesn't exist yet, initialize it
						if opts.PackageMap[distro] == nil {
							opts.PackageMap[distro] = make(map[string][]string)
						}

						// Add/update package mappings
						for pkg, mappings := range pkgMap {
							opts.PackageMap[distro][pkg] = mappings
						}
					}
				}
			}

			// Parse and convert the Dockerfile
			ctx := context.Background()
			dockerfile, err := ParseDockerfile(ctx, beforeBytes)
			if err != nil {
				t.Fatalf("ParseDockerfile() error = %v", err)
			}

			// Convert the Dockerfile
			convertedDockerfile := dockerfile.Convert(ctx, opts)

			// Get the string representation
			actual := convertedDockerfile.String()

			// Normalize line endings to ensure consistent comparison
			actual = strings.ReplaceAll(actual, "\r\n", "\n")
			expected = strings.ReplaceAll(expected, "\r\n", "\n")

			// Compare the result with the expected output
			if actual != expected {
				t.Errorf("Conversion result does not match expected output.\nGot:\n%s\n\nExpected:\n%s", actual, expected)

				// Show diff for easier debugging
				t.Logf("Diff:")
				lines1 := strings.Split(actual, "\n")
				lines2 := strings.Split(expected, "\n")

				maxLen := len(lines1)
				if len(lines2) > maxLen {
					maxLen = len(lines2)
				}

				for i := 0; i < maxLen; i++ {
					if i < len(lines1) && i < len(lines2) {
						if lines1[i] != lines2[i] {
							t.Logf("Line %d: Got %q, Expected %q", i+1, lines1[i], lines2[i])
						}
					} else if i < len(lines1) {
						t.Logf("Line %d: Got %q, Expected <none>", i+1, lines1[i])
					} else {
						t.Logf("Line %d: Got <none>, Expected %q", i+1, lines2[i])
					}
				}
			}
		})
	}
}
