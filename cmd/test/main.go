package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/chainguard-dev/dfc/pkg/dfc2"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input> <output>\n", os.Args[0])
		os.Exit(1)
	}

	input := os.Args[1]
	output := os.Args[2]

	// Read the input Dockerfile
	content, err := ioutil.ReadFile(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// Parse the Dockerfile
	ctx := context.Background()
	dockerfile, err := dfc2.ParseDockerfile(ctx, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing Dockerfile: %v\n", err)
		os.Exit(1)
	}

	// Create options
	opts := dfc2.Options{
		Organization: "chainguard",
	}

	// Convert the Dockerfile
	convertedDockerfile := dockerfile.Convert(ctx, opts)

	// Get the string representation
	result := convertedDockerfile.String()

	// Write the output
	err = ioutil.WriteFile(output, []byte(result), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Converted Dockerfile written to %s\n", output)
}
