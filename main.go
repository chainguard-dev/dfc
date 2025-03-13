package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/chainguard-dev/dfc/pkg/dfc2"
	"github.com/spf13/cobra"
)

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
				file, err := os.Open(path)
				if err != nil {
					return fmt.Errorf("failed open file: %s: %v", path, err)
				}
				defer file.Close()
				input = file
			}
			buf := new(bytes.Buffer)
			buf.ReadFrom(input)
			raw := buf.Bytes()

			// Use dfc2 to parse the Dockerfile
			dockerfile, err := dfc2.ParseDockerfile(ctx, raw)
			if err != nil {
				return fmt.Errorf("unable to parse dockerfile: %v", err)
			}

			// Setup conversion options
			opts := dfc2.Options{
				Organization: org,
				PackageMap: map[string]string{
					"ca-certificates": "ca-certificates",
					"curl":            "curl",
					"git":             "git",
					"nginx":           "nginx",
					"python3":         "python3",
					"python3-pip":     "py3-pip",
					"vim":             "vim",
				},
			}

			// Convert the Dockerfile
			convertedDockerfile := dockerfile.Convert(ctx, opts)

			if j {
				if inPlace {
					return fmt.Errorf("unable to use --in-place and --json flag at same time")
				}

				// Output the converted Dockerfile as JSON
				b, err := json.Marshal(convertedDockerfile)
				if err != nil {
					return fmt.Errorf("marshalling converted dockerfile to json: %v", err)
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
				backupPath := path + ".bak"
				log.Printf("saving dockerfile backup to %s", backupPath)
				if err := os.WriteFile(backupPath, raw, 0644); err != nil {
					return fmt.Errorf("saving dockerfile backup to %s: %v", backupPath, err)
				}
				log.Printf("overwriting %s", path)
				if err := os.WriteFile(path, []byte(result), 0644); err != nil {
					return fmt.Errorf("overwriting %s: %v", path, err)
				}
				return nil
			}

			// Print to stdout
			fmt.Print(result)

			return nil
		},
	}

	cmd.Flags().StringVar(&org, "org", dfc2.DefaultOrganization, "the organization for cgr.dev/ORGANIZATION/alpine (defaults to ORGANIZATION)")
	cmd.Flags().BoolVarP(&inPlace, "in-place", "i", false, "modified the Dockerfile in place (vs. stdout), saving original in a .bak file")
	cmd.Flags().BoolVar(&j, "json", false, "print dockerfile as json (after conversion)")

	return cmd
}
