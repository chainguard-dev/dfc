/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package apko

import (
	"testing"

	"github.com/chainguard-dev/dfc/pkg/dfc"
)

func TestConvertDockerfileToApko(t *testing.T) {
	tests := []struct {
		name       string
		dockerfile *dfc.Dockerfile
		want       *ApkoConfig
		wantErr    bool
	}{
		{
			name: "simple dockerfile",
			dockerfile: &dfc.Dockerfile{
				Lines: []*dfc.DockerfileLine{
					{
						From: &dfc.FromDetails{
							Base: "cgr.dev/ORG/alpine",
							Tag:  "latest-dev",
						},
					},
					{
						Run: &dfc.RunDetails{
							Packages: []string{"nginx"},
						},
					},
					{
						Raw: "WORKDIR /usr/share/nginx",
					},
					{
						Raw: "ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/sbin:/sbin:/bin",
					},
					{
						Raw: "USER nginx",
					},
				},
			},
			want: &ApkoConfig{
				Contents: struct {
					Repositories []string `yaml:"repositories"`
					Packages     []string `yaml:"packages"`
				}{
					Repositories: []string{"https://dl-cdn.alpinelinux.org/alpine/edge/main"},
					Packages:     []string{"alpine-base", "nginx"},
				},
				WorkDir: "/usr/share/nginx",
				Environment: map[string]string{
					"PATH": "/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/sbin:/sbin:/bin",
				},
				Accounts: struct {
					Users  []User  `yaml:"users,omitempty"`
					Groups []Group `yaml:"groups,omitempty"`
					RunAs  string  `yaml:"run-as,omitempty"`
				}{
					RunAs: "nginx",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMap, err := ConvertDockerfileToApko(tt.dockerfile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertDockerfileToApko() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Since we expect a single stage, get the first (and should be only) stage
			if len(gotMap) != 1 {
				t.Errorf("ConvertDockerfileToApko() returned %d stages, expected 1", len(gotMap))
				return
			}

			var got *ApkoConfig
			for _, config := range gotMap {
				got = config
				break
			}

			// Compare repositories
			if len(got.Contents.Repositories) != len(tt.want.Contents.Repositories) {
				t.Errorf("ConvertDockerfileToApko() repositories = %v, want %v", got.Contents.Repositories, tt.want.Contents.Repositories)
			}

			// Compare packages
			if len(got.Contents.Packages) != len(tt.want.Contents.Packages) {
				t.Errorf("ConvertDockerfileToApko() packages = %v, want %v", got.Contents.Packages, tt.want.Contents.Packages)
			}

			// Compare workdir
			if got.WorkDir != tt.want.WorkDir {
				t.Errorf("ConvertDockerfileToApko() workdir = %v, want %v", got.WorkDir, tt.want.WorkDir)
			}

			// Compare environment
			if len(got.Environment) != len(tt.want.Environment) {
				t.Errorf("ConvertDockerfileToApko() environment = %v, want %v", got.Environment, tt.want.Environment)
			}

			// Compare run-as user
			if got.Accounts.RunAs != tt.want.Accounts.RunAs {
				t.Errorf("ConvertDockerfileToApko() run-as = %v, want %v", got.Accounts.RunAs, tt.want.Accounts.RunAs)
			}
		})
	}
}

func TestGenerateApkoYAML(t *testing.T) {
	tests := []struct {
		name    string
		config  *ApkoConfig
		want    string
		wantErr bool
	}{
		{
			name: "simple config",
			config: &ApkoConfig{
				Contents: struct {
					Repositories []string `yaml:"repositories"`
					Packages     []string `yaml:"packages"`
				}{
					Repositories: []string{"https://dl-cdn.alpinelinux.org/alpine/edge/main"},
					Packages:     []string{"alpine-base", "nginx"},
				},
				WorkDir: "/usr/share/nginx",
				Environment: map[string]string{
					"PATH": "/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/sbin:/sbin:/bin",
				},
				Accounts: struct {
					Users  []User  `yaml:"users,omitempty"`
					Groups []Group `yaml:"groups,omitempty"`
					RunAs  string  `yaml:"run-as,omitempty"`
				}{
					RunAs: "nginx",
				},
			},
			want: `contents:
  repositories:
    - https://dl-cdn.alpinelinux.org/alpine/edge/main
  packages:
    - alpine-base
    - nginx
work-dir: /usr/share/nginx
environment:
  PATH: /usr/local/sbin:/usr/local/bin:/usr/bin:/usr/sbin:/sbin:/bin
accounts:
  run-as: nginx
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateApkoYAML(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateApkoYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GenerateApkoYAML() = %v, want %v", got, tt.want)
			}
		})
	}
}
