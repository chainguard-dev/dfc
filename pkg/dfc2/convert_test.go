package dfc2

import (
	"context"
	"strings"
	"testing"
)

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
				PackageMap: map[string]string{
					"nginx": "nginx",
					"curl":  "curl",
				},
			},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/alpine:latest",
				"apk add -U nginx curl",
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
				PackageMap: map[string]string{
					"ca-certificates": "ca-certificates",
				},
			},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/alpine:latest AS builder",
				"FROM cgr.dev/ORGANIZATION/alpine:latest",
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
				PackageMap: map[string]string{
					"python3":     "python3",
					"python3-pip": "py3-pip",
				},
			},
			wantContains: []string{
				"FROM cgr.dev/myorg/alpine:latest",
				"apk add -U python3 py3-pip",
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
				PackageMap: map[string]string{
					"nginx": "nginx",
					"curl":  "curl",
					"vim":   "vim",
				},
			},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/alpine:latest",
				"# Install dependencies",
				"# Run the application",
			},
		},
		{
			name: "preserves comment spacing without blank line",
			input: `FROM node:20.15.0 AS base

# my comment
ARG ABC`,
			opts: Options{},
			wantContains: []string{
				"FROM cgr.dev/ORGANIZATION/alpine:latest AS base",
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
				"FROM cgr.dev/ORGANIZATION/alpine:latest AS base",
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
		name     string
		input    string
		wantType string
		wantBase string
	}{
		{
			name:     "simple from directive",
			input:    "FROM debian:11",
			wantType: DirectiveFrom,
			wantBase: "debian",
		},
		{
			name:     "from with as",
			input:    "FROM golang:1.18 AS builder",
			wantType: DirectiveFrom,
			wantBase: "golang",
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
			}
		})
	}
}

func TestConvertFromDirective(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		opts     Options
		wantBase string
		wantKeep bool // If true, expect the base to remain unchanged
	}{
		{
			name:     "simple from directive",
			input:    "FROM debian:11",
			opts:     Options{},
			wantBase: "cgr.dev/ORGANIZATION/alpine",
			wantKeep: false,
		},
		{
			name:     "from with as",
			input:    "FROM golang:1.18 AS builder",
			opts:     Options{},
			wantBase: "cgr.dev/ORGANIZATION/alpine",
			wantKeep: false,
		},
		{
			name:     "custom organization",
			input:    "FROM ubuntu:20.04",
			opts:     Options{Organization: "myorg"},
			wantBase: "cgr.dev/myorg/alpine",
			wantKeep: false,
		},
		{
			name:     "stage reference",
			input:    "FROM builder",
			opts:     Options{},
			wantBase: "builder",
			wantKeep: true,
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

			// Apply conversion
			convertFromDirective(line, tt.opts, stageAliases)

			t.Logf("After conversion: %+v", line)
			t.Logf("Raw line: %s", line.Raw)

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
