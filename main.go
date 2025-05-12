/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chainguard-dev/clog"
	"github.com/chainguard-dev/clog/slag"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/chainguard-dev/dfc/pkg/apko"
	"github.com/chainguard-dev/dfc/pkg/dfc"
)

var (
	// Version is the semantic version (added at compile time via -X main.Version=$VERSION)
	Version string

	// Revision is the git commit id (added at compile time via -X main.Revision=$REVISION)
	Revision string

	org          = flag.String("org", "", "Organization name for Chainguard images")
	registry     = flag.String("registry", "", "Custom registry for Chainguard images")
	update       = flag.Bool("update", false, "Update mappings before conversion")
	mappingsFile = flag.String("mappings", "", "Path to custom mappings file")
	noBuiltIn    = flag.Bool("no-builtin", false, "Don't use built-in mappings")
	inPlace      = flag.Bool("in-place", false, "Convert Dockerfile in place")
	jsonOutput   = flag.Bool("json", false, "Output in JSON format")
	apkoOutput   = flag.String("apko", "", "Output path for apko overlay configuration")
	directApko   = flag.String("direct-apko", "", "Convert Dockerfile directly to apko overlay and save to the specified path")
	debugMode    = flag.Bool("debug", false, "Enable debug logging")
)

func main() {
	ctx := context.Background()

	// Parse command line arguments
	flag.Parse()

	// Set debug mode in apko package
	apko.Debug = *debugMode

	// Handle update flag
	if *update {
		if err := dfc.Update(ctx, dfc.UpdateOptions{}); err != nil {
			log.Fatalf("Failed to update mappings: %v", err)
		}
		// If only updating mappings, exit
		if flag.NArg() == 0 {
			return
		}
	}

	// Get input file path
	inputPath := flag.Arg(0)
	if inputPath == "" {
		log.Fatal("Please provide a Dockerfile path")
	}

	// Read input file
	var input []byte
	var err error
	if inputPath == "-" {
		input, err = io.ReadAll(os.Stdin)
	} else {
		input, err = os.ReadFile(inputPath)
	}
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	// Parse Dockerfile
	dockerfile, err := dfc.ParseDockerfile(ctx, input)
	if err != nil {
		log.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Convert to Chainguard format
	opts := dfc.Options{
		Organization: *org,
		Registry:     *registry,
		Update:       *update,
	}
	if *mappingsFile != "" {
		mappingsData, err := os.ReadFile(*mappingsFile)
		if err != nil {
			log.Fatalf("Failed to read mappings file: %v", err)
		}
		var mappings dfc.MappingsConfig
		if err := yaml.Unmarshal(mappingsData, &mappings); err != nil {
			log.Fatalf("Failed to parse mappings file: %v", err)
		}
		opts.ExtraMappings = mappings
	}
	if *noBuiltIn {
		opts.NoBuiltIn = true
	}

	converted, err := dockerfile.Convert(ctx, opts)
	if err != nil {
		log.Fatalf("Failed to convert Dockerfile: %v", err)
	}

	// If direct-apko is specified, generate the overlay directly and exit
	if *directApko != "" {
		// No need to print the converted Dockerfile when using direct-apko
		log.Println("Converting directly to apko overlay")

		// Parse the original Dockerfile again
		originalDockerfile, err := dfc.ParseDockerfile(ctx, input)
		if err != nil {
			log.Fatalf("Failed to parse Dockerfile for direct-apko: %v", err)
		}

		// Convert to Chainguard format specifically for apko
		convertedForApko, err := originalDockerfile.Convert(ctx, opts)
		if err != nil {
			log.Fatalf("Failed to convert Dockerfile for direct-apko: %v", err)
		}

		// Convert the Chainguard Dockerfile to apko overlay
		stageConfigs, err := apko.ConvertDockerfileToApko(convertedForApko)
		if err != nil {
			log.Fatalf("Failed to convert to apko format for direct-apko: %v", err)
		}

		if len(stageConfigs) == 0 {
			log.Println("No stages found or an error occurred during direct-apko conversion, no overlay files generated.")
			return
		}

		baseOutputDir := filepath.Dir(*directApko)
		baseOutputFileName := filepath.Base(*directApko)
		fileExt := filepath.Ext(baseOutputFileName)
		baseNameNoExt := strings.TrimSuffix(baseOutputFileName, fileExt)

		// Determine the base name from the input Dockerfile path if needed
		inputDockerfileName := "Dockerfile" // Default if input is from stdin or path is weird
		if inputPath != "" {
			inputBase := filepath.Base(inputPath)
			inputExt := filepath.Ext(inputBase)
			inputNameNoExt := strings.TrimSuffix(inputBase, inputExt)
			if inputNameNoExt != "" {
				inputDockerfileName = inputNameNoExt
			}
		}

		for stageName, stageConfig := range stageConfigs {
			// Generate apko YAML for the current stage
			currentApkoYAML, err := apko.GenerateApkoYAML(stageConfig)
			if err != nil {
				log.Fatalf("Failed to generate apko YAML for stage '%s' in direct-apko: %v", stageName, err)
			}

			var finalApkoPath string
			// Handle path construction for direct-apko output
			if baseNameNoExt == "" || baseNameNoExt == "_overlay" || baseNameNoExt == "apko" {
				finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay%s", inputDockerfileName, stageName, fileExt))
				if fileExt == "" { // if original apkoOutput had no extension
					finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay.yaml", inputDockerfileName, stageName))
				}
			} else if len(stageConfigs) > 1 || (stageName != "stage0" && stageName != "") || baseNameNoExt != inputDockerfileName {
				finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay%s", baseNameNoExt, stageName, fileExt))
				if fileExt == "" {
					finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay.yaml", baseNameNoExt, stageName))
				}
			} else {
				finalApkoPath = *directApko
			}

			// Write apko YAML to file
			if err := os.WriteFile(finalApkoPath, []byte(currentApkoYAML), 0644); err != nil {
				log.Fatalf("Failed to write apko YAML for stage '%s' to %s in direct-apko: %v", stageName, finalApkoPath, err)
			}
			log.Printf("Generated apko overlay for stage '%s' to %s", stageName, finalApkoPath)
		}

		// Exit without printing the Chainguard Dockerfile
		return
	}

	// Handle apko overlay generation
	if *apkoOutput != "" {
		// Convert the Chainguard Dockerfile to apko overlay format
		// This now returns a map of stage names to their ApkoConfig
		stageConfigs, err := apko.ConvertDockerfileToApko(converted)
		if err != nil {
			log.Fatalf("Failed to convert to apko overlay: %v", err)
		}

		if len(stageConfigs) == 0 {
			log.Println("No stages found or an error occurred during apko conversion, no overlay files generated.")
			return
		}

		baseOutputDir := filepath.Dir(*apkoOutput)
		baseOutputFileName := filepath.Base(*apkoOutput)
		fileExt := filepath.Ext(baseOutputFileName)
		baseNameNoExt := strings.TrimSuffix(baseOutputFileName, fileExt)

		// Determine the base name from the input Dockerfile path if needed
		inputDockerfileName := "Dockerfile" // Default if input is from stdin or path is weird
		if inputPath != "" {
			inputBase := filepath.Base(inputPath)
			inputExt := filepath.Ext(inputBase)
			inputNameNoExt := strings.TrimSuffix(inputBase, inputExt)
			if inputNameNoExt != "" {
				inputDockerfileName = inputNameNoExt
			}
		}

		for stageName, stageConfig := range stageConfigs {
			// Generate YAML for the current stage
			yamlOutput, err := apko.GenerateApkoYAML(stageConfig)
			if err != nil {
				log.Fatalf("Failed to generate apko YAML for stage '%s': %v", stageName, err)
			}

			// Construct the output filename for the stage
			var finalApkoPath string
			if baseNameNoExt == "" || baseNameNoExt == "_overlay" || baseNameNoExt == "apko" {
				finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay%s", inputDockerfileName, stageName, fileExt))
				if fileExt == "" { // if original apkoOutput had no extension
					finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay.yaml", inputDockerfileName, stageName))
				}
			} else if len(stageConfigs) > 1 || (stageName != "stage0" && stageName != "") || baseNameNoExt != inputDockerfileName {
				// If multiple stages, or a named stage (not default "stage0"), or if the original output name wasn't already specific to a Dockerfile.
				finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay%s", baseNameNoExt, stageName, fileExt))
				if fileExt == "" {
					finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay.yaml", baseNameNoExt, stageName))
				}
			} else { // Single stage, and a specific output name was given that likely matches the dockerfile name already.
				finalApkoPath = *apkoOutput
			}

			// Write to output file
			if err := os.WriteFile(finalApkoPath, []byte(yamlOutput), 0644); err != nil {
				log.Fatalf("Failed to write apko overlay file for stage '%s' to %s: %v", stageName, finalApkoPath, err)
			}
			fmt.Printf("Generated Apko overlay for stage '%s' to %s\n", stageName, finalApkoPath)
		}
		return
	}

	// Handle in-place conversion
	if *inPlace {
		// Create backup
		backupPath := inputPath + ".bak"
		if err := os.WriteFile(backupPath, input, 0644); err != nil {
			log.Fatalf("Failed to create backup file: %v", err)
		}

		// Write converted content
		if err := os.WriteFile(inputPath, []byte(converted.String()), 0644); err != nil {
			log.Fatalf("Failed to write converted file: %v", err)
		}
		return
	}

	// Print converted content
	if *jsonOutput {
		json, err := json.MarshalIndent(converted, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal JSON: %v", err)
		}
		fmt.Println(string(json))
	} else {
		fmt.Print(converted)
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
	var apkoOutput string
	var directApko string
	var debug bool

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

			// Set debug mode in apko package
			apko.Debug = debug

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
				Organization: org,
				Registry:     registry,
				Update:       updateFlag,
				NoBuiltIn:    noBuiltInFlag,
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

			// Print the converted Dockerfile
			fmt.Print(result)

			// If apko output is requested, convert to apko format
			if apkoOutput != "" {
				// Convert to apko format - this now returns a map of stage names to their ApkoConfig
				stageConfigs, err := apko.ConvertDockerfileToApko(convertedDockerfile)
				if err != nil {
					return fmt.Errorf("converting to apko format: %w", err)
				}

				if len(stageConfigs) == 0 {
					log.Info("No stages found or an error occurred during apko conversion, no overlay files generated.")
					return nil
				}

				baseOutputDir := filepath.Dir(apkoOutput)
				baseOutputFileName := filepath.Base(apkoOutput)
				fileExt := filepath.Ext(baseOutputFileName)
				baseNameNoExt := strings.TrimSuffix(baseOutputFileName, fileExt)

				// Determine the base name from the input Dockerfile path if needed
				inputDockerfileName := "Dockerfile" // Default if input is from stdin or path is weird
				if isFile && path != "" {
					inputBase := filepath.Base(path)
					inputExt := filepath.Ext(inputBase)
					inputNameNoExt := strings.TrimSuffix(inputBase, inputExt)
					if inputNameNoExt != "" {
						inputDockerfileName = inputNameNoExt
					}
				}

				for stageName, stageConfig := range stageConfigs {
					// Generate apko YAML for the current stage
					currentApkoYAML, err := apko.GenerateApkoYAML(stageConfig)
					if err != nil {
						return fmt.Errorf("generating apko YAML for stage '%s': %w", stageName, err)
					}

					var finalApkoPath string
					// If the original apkoOutput was generic (like "apko.yaml" or "_overlay.yaml") or just a directory,
					// base the new filename on the input Dockerfile's name.
					if baseNameNoExt == "" || baseNameNoExt == "_overlay" || baseNameNoExt == "apko" {
						finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay%s", inputDockerfileName, stageName, fileExt))
						if fileExt == "" { // if original apkoOutput had no extension
							finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay.yaml", inputDockerfileName, stageName))
						}
					} else if len(stageConfigs) > 1 || (stageName != "stage0" && stageName != "") || baseNameNoExt != inputDockerfileName {
						// If multiple stages, or a named stage (not default "stage0"), or if the original output name wasn't already specific to a Dockerfile.
						finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay%s", baseNameNoExt, stageName, fileExt))
						if fileExt == "" {
							finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay.yaml", baseNameNoExt, stageName))
						}
					} else { // Single stage, and a specific output name was given that likely matches the dockerfile name already.
						finalApkoPath = apkoOutput
					}

					// Write apko YAML to file
					if err := os.WriteFile(finalApkoPath, []byte(currentApkoYAML), 0644); err != nil {
						return fmt.Errorf("writing apko YAML for stage '%s' to %s: %w", stageName, finalApkoPath, err)
					}
					log.Info("Generated Apko overlay", "stage", stageName, "path", finalApkoPath)
				}
			}

			// If direct-apko is requested, convert the ORIGINAL Dockerfile to Chainguard format
			// and then to apko in one step (without displaying the converted Dockerfile)
			if directApko != "" {
				// IMPORTANT: We need to suppress the normal output when using direct-apko
				// We want to convert directly from raw -> chainguard -> apko without printing the Chainguard result

				// Parse the original Dockerfile
				originalDockerfile, err := dfc.ParseDockerfile(ctx, raw)
				if err != nil {
					return fmt.Errorf("unable to parse dockerfile for direct-apko: %w", err)
				}

				// Convert to Chainguard format
				convertedDockerfileForApko, err := originalDockerfile.Convert(ctx, opts)
				if err != nil {
					return fmt.Errorf("converting dockerfile for direct-apko: %w", err)
				}

				// No need to print the converted Dockerfile when using direct-apko
				log.Info("Converting directly to apko overlay")

				// Convert the Chainguard Dockerfile to apko overlay
				stageConfigs, err := apko.ConvertDockerfileToApko(convertedDockerfileForApko)
				if err != nil {
					return fmt.Errorf("converting to apko format for direct-apko: %w", err)
				}

				if len(stageConfigs) == 0 {
					log.Info("No stages found or an error occurred during direct-apko conversion, no overlay files generated.")
					return nil
				}

				baseOutputDir := filepath.Dir(directApko)
				baseOutputFileName := filepath.Base(directApko)
				fileExt := filepath.Ext(baseOutputFileName)
				baseNameNoExt := strings.TrimSuffix(baseOutputFileName, fileExt)

				// Determine the base name from the input Dockerfile path if needed
				inputDockerfileName := "Dockerfile" // Default if input is from stdin or path is weird
				if isFile && path != "" {
					inputBase := filepath.Base(path)
					inputExt := filepath.Ext(inputBase)
					inputNameNoExt := strings.TrimSuffix(inputBase, inputExt)
					if inputNameNoExt != "" {
						inputDockerfileName = inputNameNoExt
					}
				}

				for stageName, stageConfig := range stageConfigs {
					// Generate apko YAML for the current stage
					currentApkoYAML, err := apko.GenerateApkoYAML(stageConfig)
					if err != nil {
						return fmt.Errorf("generating apko YAML for stage '%s' in direct-apko: %w", stageName, err)
					}

					var finalApkoPath string
					// Handle path construction for direct-apko output
					if baseNameNoExt == "" || baseNameNoExt == "_overlay" || baseNameNoExt == "apko" {
						finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay%s", inputDockerfileName, stageName, fileExt))
						if fileExt == "" { // if original apkoOutput had no extension
							finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay.yaml", inputDockerfileName, stageName))
						}
					} else if len(stageConfigs) > 1 || (stageName != "stage0" && stageName != "") || baseNameNoExt != inputDockerfileName {
						finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay%s", baseNameNoExt, stageName, fileExt))
						if fileExt == "" {
							finalApkoPath = filepath.Join(baseOutputDir, fmt.Sprintf("%s_%s_overlay.yaml", baseNameNoExt, stageName))
						}
					} else {
						finalApkoPath = directApko
					}

					// Write apko YAML to file
					if err := os.WriteFile(finalApkoPath, []byte(currentApkoYAML), 0644); err != nil {
						return fmt.Errorf("writing apko YAML for stage '%s' to %s in direct-apko: %w", stageName, finalApkoPath, err)
					}
					log.Info("Generated Apko overlay directly from original Dockerfile", "stage", stageName, "path", finalApkoPath)
				}

				// Return here to prevent printing the Chainguard Dockerfile
				return nil
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&j, "json", "j", false, "output as json")
	cmd.Flags().BoolVarP(&inPlace, "in-place", "i", false, "modify file in place")
	cmd.Flags().StringVar(&org, "org", dfc.DefaultOrg, "organization name for cgr.dev namespace")
	cmd.Flags().StringVar(&registry, "registry", dfc.DefaultRegistryDomain, "registry domain and root namespace")
	cmd.Flags().StringVar(&mappingsFile, "mappings", "", "path to custom mappings file")
	cmd.Flags().BoolVar(&updateFlag, "update", false, "update cached mappings")
	cmd.Flags().BoolVar(&noBuiltInFlag, "no-builtin", false, "don't use built-in mappings")
	cmd.Flags().StringVar(&apkoOutput, "apko", "", "output path for apko configuration file")
	cmd.Flags().StringVar(&directApko, "direct-apko", "", "convert Dockerfile directly to apko overlay and save to the specified path")
	cmd.Flags().BoolVar(&debug, "debug", false, "enable debug logging")
	cmd.Flags().Var(&level, "log-level", "log level (e.g. debug, info, warn, error)")

	return cmd
}
