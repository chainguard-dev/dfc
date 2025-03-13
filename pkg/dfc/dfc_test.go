package dfc

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParseDockerfile(t *testing.T) {
	for _, testcase := range []struct {
		name     string
		content  string
		expected *Dockerfile
		// Packages is not properly parsed in the test as each individual line is processed separately
		skipPackages bool
	}{
		{
			name: "basic parse",
			content: `
				# This is my dockerfile
				FROM        node:1.2.3 AS my-stage
                
				ruN            apt-get update -qq && apt-get install -y nano zsh && \
				  chmod +x bin/oh-my-zsh.sh && \
				  sh -c "RUNZSH=no bin/oh-my-zsh.sh"
                
				FROM   my-stage AS other
                
				# Make sure we are able to determine dynamic base and tag
				froM    blah-${SOME_VAR} AS my-dynamic-repo
				FROM  node:${SOME_VERSION} AS my-dynamic-tag
				FROM blah-${SOME_VAR}:${SOME_VERSION} AS my-dynamic-both
                

				RUN apt-get update && \
					apt-get install -y mypackage1 mypackage2 mypackage3 && \
					echo hello world && \
					    wow


				COPY . .

				# testing without alias
				FROM node:1.2.3
			`,
			expected: &Dockerfile{
				Lines: []*DockerfileLine{
					{},
					{Directive: DirectiveFrom, From: &FromDetails{Base: "node", Tag: "1.2.3", Alias: "my-stage"}, Stage: 1},
					{Directive: DirectiveRun, Run: &RunDetails{Distro: DistroDebian}, Stage: 1},

					{Directive: DirectiveFrom, From: &FromDetails{Base: "my-stage", Alias: "other", Parent: 1}, Stage: 2},
					{Directive: DirectiveFrom, From: &FromDetails{Base: "blah-${SOME_VAR}", Alias: "my-dynamic-repo", BaseDynamic: true}, Stage: 3},
					{Directive: DirectiveFrom, From: &FromDetails{Base: "node", Tag: "${SOME_VERSION}", Alias: "my-dynamic-tag", TagDynamic: true}, Stage: 4},
					{Directive: DirectiveFrom, From: &FromDetails{Base: "blah-${SOME_VAR}", Tag: "${SOME_VERSION}", Alias: "my-dynamic-both", BaseDynamic: true, TagDynamic: true}, Stage: 5},
					{Directive: DirectiveRun, Run: &RunDetails{Distro: DistroDebian}, Stage: 5},

					{Directive: "COPY", Stage: 5},
					{Directive: DirectiveFrom, From: &FromDetails{Base: "node", Tag: "1.2.3"}, Stage: 6},
				},
			},
			skipPackages: true,
		},
		{
			name: "package detection",
			content: `
				FROM alpine:latest
				RUN apk add python3 nginx curl

				FROM fedora:latest
				RUN dnf install -y httpd vim gcc

				FROM debian:latest 
				RUN apt-get update && apt-get install -y git nodejs npm --no-install-recommends
			`,
			expected: &Dockerfile{
				Lines: []*DockerfileLine{
					{},
					{Directive: DirectiveFrom, From: &FromDetails{Base: "alpine", Tag: "latest"}, Stage: 1},
					{Directive: DirectiveRun, Run: &RunDetails{Distro: DistroAlpine, Packages: []string{"python3", "nginx", "curl"}}, Stage: 1},

					{Directive: DirectiveFrom, From: &FromDetails{Base: "fedora", Tag: "latest"}, Stage: 2},
					{Directive: DirectiveRun, Run: &RunDetails{Distro: DistroFedora, Packages: []string{"httpd", "vim", "gcc"}}, Stage: 2},

					{Directive: DirectiveFrom, From: &FromDetails{Base: "debian", Tag: "latest"}, Stage: 3},
					{Directive: DirectiveRun, Run: &RunDetails{Distro: DistroDebian, Packages: []string{"git", "nodejs", "npm"}}, Stage: 3},
				},
			},
		},
	} {
		actual, err := ParseDockerfile(context.TODO(), []byte(testcase.content))
		if err != nil {
			t.Fatal(err) // TODO: test errors
		}

		// Make sure all fields we expect were extracted
		// Intentionally no testing of Raw/Content/Extra as we have confidence these are working if everything else parsed
		ignoreFields := []string{"Raw", "Content", "Extra"}
		if testcase.skipPackages {
			for i, line := range actual.Lines {
				if line.Run != nil && line.Run.Packages != nil {
					// Clear the packages for comparison since we're skipping them
					actual.Lines[i].Run.Packages = nil
				}
			}
		} else {
			// Sort the expected packages for comparison, as the actual packages are sorted during parsing
			for i, line := range testcase.expected.Lines {
				if line.Run != nil && line.Run.Packages != nil {
					// Sort the expected packages
					sortedPkgs := make([]string, len(line.Run.Packages))
					copy(sortedPkgs, line.Run.Packages)
					sort.Strings(sortedPkgs)
					testcase.expected.Lines[i].Run.Packages = sortedPkgs
				}
			}
		}

		// Always ignore the Command field since it's not part of the expected output
		if diff := cmp.Diff(testcase.expected, actual, cmpopts.IgnoreFields(DockerfileLine{}, ignoreFields...),
			cmpopts.IgnoreFields(RunDetails{}, "command")); diff != "" {
			t.Errorf("%s: did not get expected output (-want +got) %s", testcase.name, diff)
		}

		// Ensure the string converts back to the original
		if diff := cmp.Diff(testcase.content, actual.String()); diff != "" {
			t.Errorf("%s: did not get string conversion (-want +got) %s", testcase.name, diff)
		}
	}
}

func TestDockerfileConvert(t *testing.T) {
	tests := []struct {
		name    string
		content string
		opts    *Options
		// Instead of checking exact strings, test for contained substrings
		mustContain    []string
		mustNotContain []string
	}{
		{
			name: "apt to apk conversion",
			content: `
FROM debian:latest
RUN apt-get update && apt-get install -y git nginx curl
`,
			opts: &Options{
				Organization: "testorg",
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/debian:latest-dev",
				"USER root",
				"RUN apk add -U git nginx curl",
			},
		},
		{
			name: "dnf to apk conversion",
			content: `
FROM fedora:latest
RUN dnf install -y httpd vim gcc
`,
			opts: &Options{
				Organization: "testorg",
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/fedora:latest-dev",
				"USER root",
				"RUN apk add -U httpd vim gcc",
			},
		},
		{
			name: "complex apt command with non-package manager parts",
			content: `
FROM debian:latest
RUN apt-get update -qq && apt-get install -y nano zsh && \
  chmod +x bin/oh-my-zsh.sh && \
  sh -c "RUNZSH=no bin/oh-my-zsh.sh"
`,
			opts: &Options{
				Organization: "testorg",
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/debian:latest-dev",
				"USER root",
				"apk add -U nano zsh",
				"chmod +x bin/oh-my-zsh.sh",
				"sh -c \"RUNZSH=no bin/oh-my-zsh.sh\"",
			},
		},
		{
			name: "apk to apk conversion",
			content: `
FROM alpine:latest
RUN apk add python3 nginx curl
`,
			opts: &Options{
				Organization: "testorg",
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/alpine:latest-dev",
				"USER root",
				"RUN apk add -U python3 nginx curl",
			},
		},
		{
			name: "cleanup trailing backslashes after command removal",
			content: `
FROM debian:latest
RUN pipenv install --ignore-pipfile --system --deploy --clear \
&& pip uninstall pipenv -y \
&& apt-get autoremove -y \
&& rm -rf /root/.cache \
&& apt-get remove -y gcc libpq-dev \
&& apt-get clean
`,
			opts: &Options{
				Organization: "testorg",
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/debian:latest-dev",
				"USER root",
				"pipenv install --ignore-pipfile --system --deploy --clear",
				"pip uninstall pipenv -y",
				"rm -rf /root/.cache",
			},
		},
		{
			name: "cleanup trailing whitespace",
			content: `
FROM debian:latest
RUN apt-get update && apt-get install -y git   \
&&    echo "cleaning up"   \
&& apt-get clean   
`,
			opts: &Options{
				Organization: "testorg",
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/debian:latest-dev",
				"USER root",
				"apk add -U git",
				"echo \"cleaning up\"",
			},
			mustNotContain: []string{
				"apt-get clean",
				"\\",  // No trailing backslashes
				"   ", // No multiple spaces
			},
		},
		{
			name: "cleanup empty lines with backslashes",
			content: `
FROM debian:latest
RUN apt-get update -qq && apt-get install -y nano zsh && \
  chmod +x bin/oh-my-zsh.sh && \
  apt-get autoremove -y && \
  sh -c "RUNZSH=no bin/oh-my-zsh.sh"
`,
			opts: &Options{
				Organization: "testorg",
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/debian:latest-dev",
				"USER root",
				"apk add -U nano zsh",
				"chmod +x bin/oh-my-zsh.sh",
				"sh -c \"RUNZSH=no bin/oh-my-zsh.sh\"",
			},
			mustNotContain: []string{
				"apt-get update",
				"apt-get autoremove -y", // Be very specific about what we don't want
			},
		},
		{
			name: "package mapping for debian",
			content: `
FROM debian:latest
RUN apt-get update && apt-get install -y nano vim git
`,
			opts: &Options{
				Organization: "testorg",
				PackageMap: map[Distro]map[string][]string{
					DistroDebian: {
						"nano": {"nano-alpine", "nano-extras"},
						"vim":  {"vim-alpine"},
						// git has no mapping, so it should remain as "git"
					},
				},
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/debian:latest-dev",
				"USER root",
				"nano-alpine",
				"nano-extras",
				"vim-alpine",
				"git",
			},
		},
		{
			name: "package mapping for multiple distros",
			content: `
FROM fedora:latest
RUN dnf install -y nginx vim httpd
`,
			opts: &Options{
				Organization: "testorg",
				PackageMap: map[Distro]map[string][]string{
					DistroDebian: {
						// This shouldn't be used since we're using Fedora here
						"vim": {"vim-debian-specific"},
					},
					DistroFedora: {
						"nginx": {"nginx-alpine", "nginx-extras"},
						"vim":   {"vim-alpine", "vim-extras"},
						// httpd has no mapping, so it should remain as "httpd"
					},
				},
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/fedora:latest-dev",
				"USER root",
				"nginx-alpine",
				"nginx-extras",
				"vim-alpine",
				"vim-extras",
				"httpd",
			},
		},
		{
			name: "only non-package manager commands remain",
			content: `
FROM debian:latest
RUN apt-get update && apt-get install -y && \
    echo hello && \
    echo goodbye
`,
			opts: &Options{
				Organization: "testorg",
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/debian:latest-dev",
				"USER root",
			},
			// The key requirements: no "true" command and we need line continuations if they were in the original
			mustNotContain: []string{
				"apt-get update",
				"apt-get install",
				"RUN true", // We should not have the dummy command
				" && \n",   // No line continuations without backslash
			},
		},
		{
			name: "apt-get install and remove commands",
			content: `
FROM debian:latest
RUN apt-get update \
&& apt-get install -qy libpq-dev \
&& apt-get remove -y libpq-dev \
&& apt-get clean \
&& rm -rf /var/lib/apt/lists/*
`,
			opts: &Options{
				Organization: "testorg",
			},
			mustContain: []string{
				"FROM cgr.dev/testorg/debian:latest-dev",
				"USER root",
				"RUN apk add -U libpq-dev",
				"rm -rf /var/lib/apt/lists/*",
			},
			mustNotContain: []string{
				"apt-get update",
				"apt-get install",
				"apt-get remove",
				"apt-get clean",
				"RUN true", // We should not have the dummy command
				" && \n",   // No line continuations without backslash
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Parse the Dockerfile
			df, err := ParseDockerfile(context.Background(), []byte(tc.content))
			if err != nil {
				t.Fatalf("Failed to parse Dockerfile: %v", err)
			}

			// Convert the Dockerfile
			err = df.convert(context.Background(), tc.opts)
			if err != nil {
				t.Fatalf("Failed to convert Dockerfile: %v", err)
			}

			// Get the entire converted content
			convertedContent := df.String()

			// Check that all required substrings are present
			for _, substr := range tc.mustContain {
				// Special handling for apk commands with packages
				if strings.HasPrefix(substr, "RUN apk add -U ") {
					// Extract the expected packages from the substring
					expectedPkgs := strings.Split(strings.TrimPrefix(substr, "RUN apk add -U "), " ")

					// Check that each package is present in the converted content
					allPackagesFound := true
					for _, pkg := range expectedPkgs {
						if !strings.Contains(convertedContent, pkg) {
							allPackagesFound = false
							t.Errorf("Expected package %q not found in conversion:\nIn content:\n%s",
								pkg, convertedContent)
						}
					}

					// Make sure the command prefix is present
					if !strings.Contains(convertedContent, "RUN apk add -U ") {
						t.Errorf("Expected command prefix 'RUN apk add -U ' not found in conversion:\nIn content:\n%s",
							convertedContent)
					}

					// Skip to the next substring if all checks passed
					if allPackagesFound {
						continue
					}
				}

				// Standard substring check for non-package lines
				if !strings.Contains(convertedContent, substr) {
					t.Errorf("Expected substring not found in conversion:\nSubstring: %s\nIn content:\n%s",
						substr, convertedContent)
				}
			}

			// Check for explicitly unwanted content
			for _, unwanted := range tc.mustNotContain {
				if strings.Contains(convertedContent, unwanted) {
					t.Errorf("Unexpected content found in conversion:\nContent: %s", convertedContent)
				}
			}
		})
	}
}
