package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainguard-dev/dfc/pkg/dfc2"
)

func main() {
	// Check if we're in debug mode
	debugShellParser()

	// Define command-line flags
	inputFile := flag.String("input", "", "Input Dockerfile path")
	outputFile := flag.String("output", "", "Output Dockerfile path (defaults to stdout)")
	org := flag.String("org", dfc2.DefaultOrganization, "Organization for base image at cgr.dev/ORGANIZATION/alpine (defaults to ORGANIZATION)")
	packageMapFile := flag.String("package-map", "", "Path to package mapping JSON file")
	debug := flag.Bool("debug", false, "Enable debug output")

	flag.Parse()

	// Check for required flags
	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: input file is required")
		flag.Usage()
		os.Exit(1)
	}

	// Read input file
	content, err := ioutil.ReadFile(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// Create options
	opts := dfc2.Options{
		Organization: *org,
		PackageMap:   make(map[string]string),
	}

	// Load package map if provided
	if *packageMapFile != "" {
		packageMap, err := loadPackageMap(*packageMapFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading package map: %v\n", err)
			os.Exit(1)
		}
		opts.PackageMap = packageMap
	} else {
		// Use some default mappings
		opts.PackageMap = map[string]string{
			"ca-certificates": "ca-certificates",
			"curl":            "curl",
			"git":             "git",
			"nginx":           "nginx",
			"python3":         "python3",
			"python3-pip":     "py3-pip",
			"vim":             "vim",
		}
	}

	// Convert the Dockerfile
	ctx := context.Background()
	var result []byte
	if *debug {
		result, err = dfc2.DebugConvertDockerfile(ctx, content, opts)
	} else {
		result, err = dfc2.ConvertDockerfile(ctx, content, opts)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting Dockerfile: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if *outputFile == "" {
		// Write to stdout
		fmt.Print(string(result))
	} else {
		// Create output directory if it doesn't exist
		outputDir := filepath.Dir(*outputFile)
		if outputDir != "." {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
				os.Exit(1)
			}
		}

		// Write to file
		if err := ioutil.WriteFile(*outputFile, result, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Converted Dockerfile written to %s\n", *outputFile)
	}
}

// loadPackageMap loads a package mapping from a file
// The file format is simple: one mapping per line, source=target
func loadPackageMap(path string) (map[string]string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	packageMap := make(map[string]string)
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		source := strings.TrimSpace(parts[0])
		target := strings.TrimSpace(parts[1])
		if source != "" && target != "" {
			packageMap[source] = target
		}
	}

	return packageMap, nil
}
