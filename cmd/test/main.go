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

	// Create options
	opts := dfc2.Options{
		Organization: "chainguard",
	}

	// Convert the Dockerfile
	ctx := context.Background()
	result, err := dfc2.ConvertDockerfile(ctx, content, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting Dockerfile: %v\n", err)
		os.Exit(1)
	}

	// Write the output
	err = ioutil.WriteFile(output, []byte(result), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Converted Dockerfile written to %s\n", output)
}
