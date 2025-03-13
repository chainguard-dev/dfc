package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/chainguard-dev/dfc/pkg/dfc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

//go:embed images.yaml
var imagesYamlBytes []byte

//go:embed packages.yaml
var packagesYamlBytes []byte

func main() {
	// inspired by https://github.com/jonjohnsonjr/apkrane/blob/main/main.go
	if err := cli().ExecuteContext(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func cli() *cobra.Command {
	var j bool
	var inPlace bool
	var org string
	var mappingsFile string

	cmd := &cobra.Command{
		Use:     "dfc",
		Example: "dfc <path_to_dockerfile>",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Allow for piping into the CLI
			var input io.Reader = cmd.InOrStdin()
			isFile := len(args) > 0 && args[0] != "-"
			var path string
			if isFile {
				path = args[0]
				file, err := os.Open(filepath.Clean(path))
				if err != nil {
					return fmt.Errorf("failed open file: %s: %v", path, err)
				}
				defer file.Close()
				input = file
			}
			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(input); err != nil {
				return fmt.Errorf("failed to read input: %v", err)
			}
			raw := buf.Bytes()

			// Use dfc2 to parse the Dockerfile
			dockerfile, err := dfc.ParseDockerfile(ctx, raw)
			if err != nil {
				return fmt.Errorf("unable to parse dockerfile: %v", err)
			}

			if j {
				if inPlace {
					return fmt.Errorf("unable to use --in-place and --json flag at same time")
				}

				// Output the Dockerfile as JSON
				b, err := json.Marshal(dockerfile)
				if err != nil {
					return fmt.Errorf("marshalling dockerfile to json: %v", err)
				}
				fmt.Println(string(b))
				return nil
			}

			// Load image mappings from embedded images.yaml
			var imageMap dfc.ImageMap

			// Parse the directory listing format in the embedded images.yaml
			var imgYaml struct {
				Directory []string `yaml:"directory"`
			}
			if err := yaml.Unmarshal(imagesYamlBytes, &imgYaml); err != nil {
				return fmt.Errorf("unmarshalling images.yaml: %v", err)
			}

			// Convert the directory list to our ImageMap format
			for _, imageName := range imgYaml.Directory {
				imageMap.Mappings = append(imageMap.Mappings, dfc.ImageMapping{
					Source: imageName,
					Target: imageName,
				})
			}

			// Try to parse and merge additional mappings from packages.yaml or custom mappings file
			var packageMap dfc.PackageMap
			var mappingsBytes []byte

			// Use custom mappings file if provided
			if mappingsFile != "" {
				var err error
				mappingsBytes, err = os.ReadFile(mappingsFile)
				if err != nil {
					return fmt.Errorf("reading mappings file %s: %v", mappingsFile, err)
				}
				log.Printf("using custom mappings file: %s", mappingsFile)
			} else {
				// Use embedded packages.yaml
				mappingsBytes = packagesYamlBytes
			}

			if err := yaml.Unmarshal(mappingsBytes, &packageMap); err != nil {
				return fmt.Errorf("unmarshalling package mappings: %v", err)
			}

			// Setup conversion options
			opts := dfc.Options{
				Organization: org,
				PackageMap:   packageMap,
				ImageMap:     imageMap,
			}

			// Convert the Dockerfile
			convertedDockerfile := dockerfile.Convert(ctx, opts)

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
					return fmt.Errorf("getting file info for %s: %v", path, err)
				}
				originalMode := fileInfo.Mode().Perm()

				backupPath := path + ".bak"
				log.Printf("saving dockerfile backup to %s", backupPath)
				if err := os.WriteFile(backupPath, raw, originalMode); err != nil {
					return fmt.Errorf("saving dockerfile backup to %s: %v", backupPath, err)
				}
				log.Printf("overwriting %s", path)
				if err := os.WriteFile(path, []byte(result), originalMode); err != nil {
					return fmt.Errorf("overwriting %s: %v", path, err)
				}
				return nil
			}

			// Print to stdout
			fmt.Print(result)

			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", dfc.DefaultOrganization, "the organization for cgr.dev/ORGANIZATION/<image> Chainguard images (defaults to ORGANIZATION)")
	cmd.Flags().BoolVarP(&inPlace, "in-place", "i", false, "modified the Dockerfile in place (vs. stdout), saving original in a .bak file")
	cmd.Flags().BoolVarP(&j, "json", "j", false, "print dockerfile as json (before conversion)")
	cmd.Flags().StringVarP(&mappingsFile, "mappings", "m", "", "path to a custom package mappings YAML file (instead of the default)")

	return cmd
}
