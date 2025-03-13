package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/chainguard-dev/dfc/pkg/dfc"
)

var (
	raw = []byte(strings.TrimSpace(`
		FROM node
		RUN apt-get update && apt-get install -y nano
	`))

	org = "example.com"
)

func main() {
	ctx := context.Background()

	// Parse the Dockefile bytes
	dockerfile, err := dfc.ParseDockerfile(ctx, raw)
	if err != nil {
		log.Fatalf("ParseDockerfile(): %v", err)
	}

	// Convert
	if err := dockerfile.Convert(ctx, dfc.Options{
		Organization: org,
	}); err != nil {
		log.Fatalf("Convert(): %v", err)
	}

	// Print converted Dockerfile content
	fmt.Println(dockerfile)
}
