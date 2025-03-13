# DFC2 - Dockerfile Converter

DFC2 is a package for converting Dockerfiles to use Alpine Linux as the base image. It provides a clean, modular approach to Dockerfile conversion with a focus on maintainability and extensibility.

## Features

- Converts Dockerfiles from Debian, Ubuntu, and Fedora-based images to Alpine Linux
- Handles multi-stage builds
- Maps packages from the source distribution to Alpine equivalents
- Preserves formatting and comments in the Dockerfile
- Supports custom organization for the base image

## Architecture

DFC2 is built with a clean separation of concerns:

1. **Shell Command Parsing**: The `internal/shellparse2` package provides a robust shell command parser that can parse, manipulate, and reconstruct shell commands while preserving formatting.

2. **Dockerfile Parsing**: The `pkg/dfc2` package provides a Dockerfile parser that can parse Dockerfiles into a structured representation.

3. **Dockerfile Conversion**: The `pkg/dfc2` package also provides a Dockerfile converter that can convert Dockerfiles to use Alpine Linux.

## Usage

```go
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/chainguard-dev/dfc/pkg/dfc2"
)

func main() {
	// Read the Dockerfile
	content, err := ioutil.ReadFile("Dockerfile")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading Dockerfile: %v\n", err)
		os.Exit(1)
	}

	// Create options
	opts := dfc2.Options{
		Organization: "myorg", // Optional: Use a custom organization for the base image
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
	ctx := context.Background()
	result, err := dfc2.ConvertDockerfile(ctx, content, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting Dockerfile: %v\n", err)
		os.Exit(1)
	}

	// Write the result
	if err := ioutil.WriteFile("Dockerfile.alpine", result, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing Dockerfile.alpine: %v\n", err)
		os.Exit(1)
	}
}
```

## CLI Usage

The `dfc2` command-line tool provides a simple interface to the DFC2 package:

```
$ dfc2 -input Dockerfile -output Dockerfile.alpine
```

Options:
- `-input`: The input Dockerfile path (required)
- `-output`: The output Dockerfile path (defaults to stdout)
- `-org`: Organization for base image (e.g., 'myorg')
- `-package-map`: Path to package mapping file
- `-debug`: Enable debug output

## Package Mapping

The package mapping file is a simple text file with one mapping per line in the format `source=target`. For example:

```
ca-certificates=ca-certificates
curl=curl
git=git
nginx=nginx
python3=python3
python3-pip=py3-pip
vim=vim
```

If no package mapping file is provided, a default mapping is used for common packages. 