package test_client

import (
	"context"
	"log"
	"os"

	"github.com/chainguard-dev/dfc/pkg/dfc"
)

func main() {
	logger := log.New(os.Stdout, "[test-client] ", log.LstdFlags)
	logger.Println("Testing DFC conversion functions directly")

	// Test Dockerfile content
	dockerfileContent := `FROM alpine:3.18
RUN apk add --no-cache curl wget gcc python3
WORKDIR /app
COPY . .
CMD ["python3", "app.py"]`

	// Write test Dockerfile to disk for reference
	err := os.WriteFile("test-dockerfile.txt", []byte(dockerfileContent), 0644)
	if err != nil {
		logger.Fatalf("Failed to write test Dockerfile: %v", err)
	}

	// Test direct conversion
	logger.Println("Testing Dockerfile conversion:")
	logger.Println("Original Dockerfile:")
	logger.Println(dockerfileContent)

	ctx := context.Background()

	// Parse the Dockerfile
	logger.Println("\nParsing Dockerfile...")
	dockerfile, err := dfc.ParseDockerfile(ctx, []byte(dockerfileContent))
	if err != nil {
		logger.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Convert the Dockerfile
	logger.Println("Converting Dockerfile...")
	opts := dfc.Options{
		Organization: "example",
	}

	converted, err := dockerfile.Convert(ctx, opts)
	if err != nil {
		logger.Fatalf("Failed to convert Dockerfile: %v", err)
	}

	// Print the converted Dockerfile
	logger.Println("\nConverted Dockerfile:")
	logger.Println(converted.String())

	// Test Dockerfile analysis
	logger.Println("\nAnalyzing Dockerfile structure:")

	stageCount := 0
	baseImages := []string{}
	packageManagers := map[string]bool{}

	for _, line := range dockerfile.Lines {
		if line.From != nil {
			stageCount++
			if line.From.Orig != "" {
				baseImages = append(baseImages, line.From.Orig)
			} else {
				baseImg := line.From.Base
				if line.From.Tag != "" {
					baseImg += ":" + line.From.Tag
				}
				baseImages = append(baseImages, baseImg)
			}
		}
		if line.Run != nil && line.Run.Manager != "" {
			packageManagers[string(line.Run.Manager)] = true
		}
	}

	// Print analysis results
	logger.Printf("- Total stages: %d\n", stageCount)
	logger.Printf("- Base images: %v\n", baseImages)

	logger.Println("- Package managers:")
	for pm := range packageManagers {
		logger.Printf("  - %s\n", pm)
	}

	logger.Println("\nDFC conversion test completed successfully")

	// Write the converted Dockerfile to disk
	err = os.WriteFile("converted-dockerfile.txt", []byte(converted.String()), 0644)
	if err != nil {
		logger.Fatalf("Failed to write converted Dockerfile: %v", err)
	}
	logger.Println("Converted Dockerfile written to converted-dockerfile.txt")
}
