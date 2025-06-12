/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/chainguard-dev/clog"
	"github.com/chainguard-dev/clog/slag"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/chainguard-dev/dfc/pkg/dfc"
)

var (
	// Version is the semantic version (added at compile time via -X main.Version=$VERSION)
	Version string

	// Revision is the git commit id (added at compile time via -X main.Revision=$REVISION)
	Revision string
)

func main() {
	ctx := context.Background()
	if err := mainE(ctx); err != nil {
		clog.FromContext(ctx).Fatal(err.Error())
	}
}

func mainE(ctx context.Context) error {
	ctx, done := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer done()
	return cli().ExecuteContext(ctx)
}

func cli() *cobra.Command {
	var j bool
	var inPlace bool
	var org string
	var registry string
	var mappingsFile string
	var updateFlag bool
	var noBuiltInFlag bool
	var multistageFlag bool
	var strictFlag bool
	var warnMissingPackagesFlag bool

	// Default log level is info
	var level = slag.Level(slog.LevelInfo)

	v := "dev"
	if Version != "" {
		v = Version
		if Revision != "" {
			v += fmt.Sprintf(" (%s)", Revision)
		}
	}

	cmd := &cobra.Command{
		Use:     "dfc",
		Example: "dfc <path_to_dockerfile>",
		Args:    cobra.MaximumNArgs(1),
		Version: v,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Setup logging
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &level})))
			log := clog.New(slog.Default().Handler())
			ctx := clog.WithLogger(cmd.Context(), log)

			// If update flag is set but no args, just update and exit
			if updateFlag && len(args) == 0 {
				// Set up update options
				updateOpts := dfc.UpdateOptions{}

				// Set UserAgent if version info is available
				if Version != "" {
					updateOpts.UserAgent = "dfc/" + Version
				}

				if err := dfc.Update(ctx, updateOpts); err != nil {
					return fmt.Errorf("failed to update: %w", err)
				}
				return nil
			}

			// If no args and no update flag, require an argument
			if len(args) == 0 {
				return fmt.Errorf("requires at least 1 arg(s), only received 0")
			}

			// Allow for piping into the CLI if first arg is "-"
			input := cmd.InOrStdin()
			isFile := args[0] != "-"
			var path string
			if isFile {
				path = args[0]
				file, err := os.Open(filepath.Clean(path))
				if err != nil {
					return fmt.Errorf("failed open file: %s: %w", path, err)
				}
				defer file.Close()
				input = file
			}
			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(input); err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			raw := buf.Bytes()

			// Use dfc2 to parse the Dockerfile
			dockerfile, err := dfc.ParseDockerfile(ctx, raw)
			if err != nil {
				return fmt.Errorf("unable to parse dockerfile: %w", err)
			}

			// Setup conversion options
			opts := dfc.Options{
				Organization:        org,
				Registry:            registry,
				Update:              updateFlag,
				NoBuiltIn:           noBuiltInFlag,
				ConvertToMultistage: multistageFlag,
				Strict:       strictFlag,
				WarnMissingPackages: warnMissingPackagesFlag,
			}

			// If custom mappings file is provided, load it as ExtraMappings
			if mappingsFile != "" {
				log.Info("Loading custom mappings file", "file", mappingsFile)
				mappingsBytes, err := os.ReadFile(mappingsFile)
				if err != nil {
					return fmt.Errorf("reading mappings file %s: %w", mappingsFile, err)
				}

				var extraMappings dfc.MappingsConfig
				if err := yaml.Unmarshal(mappingsBytes, &extraMappings); err != nil {
					return fmt.Errorf("unmarshalling package mappings: %w", err)
				}

				opts.ExtraMappings = extraMappings
			}

			// If --no-builtin flag is used without --mappings, warn the user
			if noBuiltInFlag && mappingsFile == "" {
				log.Warn("Using --no-builtin without --mappings will use default conversion logic without any package/image mappings")
			}

			// Convert the Dockerfile
			convertedDockerfile, err := dockerfile.Convert(ctx, opts)
			if err != nil {
				return fmt.Errorf("converting dockerfile: %w", err)
			}

			// Output the Dockerfile as JSON
			if j {
				if inPlace {
					return fmt.Errorf("unable to use --in-place and --json flag at same time")
				}

				// Output the Dockerfile as JSON
				b, err := json.Marshal(convertedDockerfile)
				if err != nil {
					return fmt.Errorf("marshalling dockerfile to json: %w", err)
				}
				fmt.Println(string(b))
				return nil
			}

			// Get the string representation
			result := convertedDockerfile.String()

			// modify file in place
			if inPlace {
				if !isFile {
					return fmt.Errorf("unable to use --in-place flag when processing stdin")
				}

				// Get original file info to preserve permissions
				fileInfo, err := os.Stat(path)
				if err != nil {
					return fmt.Errorf("getting file info for %s: %w", path, err)
				}
				originalMode := fileInfo.Mode().Perm()

				backupPath := path + ".bak"
				log.Info("Saving dockerfile backup", "path", backupPath)
				if err := os.WriteFile(backupPath, raw, originalMode); err != nil {
					return fmt.Errorf("saving dockerfile backup to %s: %w", backupPath, err)
				}
				log.Info("Overwriting dockerfile", "path", path)
				if err := os.WriteFile(path, []byte(result), originalMode); err != nil {
					return fmt.Errorf("overwriting %s: %w", path, err)
				}
				return nil
			}

			// Print to stdout
			fmt.Print(result)

			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", dfc.DefaultOrg, "the organization for cgr.dev/<org>/<image> (defaults to ORG)")
	cmd.Flags().StringVar(&registry, "registry", "", "an alternate registry and root namepace (e.g. r.example.com/cg-mirror)")
	cmd.Flags().BoolVarP(&inPlace, "in-place", "i", false, "modified the Dockerfile in place (vs. stdout), saving original in a .bak file")
	cmd.Flags().BoolVarP(&j, "json", "j", false, "print dockerfile as json (before conversion)")
	cmd.Flags().StringVarP(&mappingsFile, "mappings", "m", "", "path to a custom package mappings YAML file (instead of the default)")
	cmd.Flags().BoolVar(&updateFlag, "update", false, "check for and apply available updates")
	cmd.Flags().BoolVar(&noBuiltInFlag, "no-builtin", false, "skip built-in package/image mappings, still apply default conversion logic")
	cmd.Flags().BoolVar(&multistageFlag, "multistage", false, "convert single-stage Dockerfiles to secure multistage builds")
	cmd.Flags().Var(&level, "log-level", "log level (e.g. debug, info, warn, error)")
	cmd.Flags().BoolVar(&strictFlag, "strict", false, "when true, fail if any package is unknown")
	cmd.Flags().BoolVar(&warnMissingPackagesFlag, "warn-missing-packages", false, "when true, warn about missing package mappings")

	return cmd
}
