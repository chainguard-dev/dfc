package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chainguard-dev/dfc/pkg/dfc"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create an MCP server instance
	s := server.NewMCPServer(
		"DFC - Extended Dockerfile Converter",
		"1.0.0",
		server.WithLogging(),
		server.WithRecovery(),
		server.WithResourceCapabilities(true, true),
	)

	// Define the Dockerfile converter tool
	s.AddTool(
		mcp.NewTool("convert_dockerfile",
			mcp.WithDescription("Convert a Dockerfile to use Chainguard Images and APKs in FROM and RUN lines"),
			mcp.WithString("dockerfile_content",
				mcp.Required(),
				mcp.Description("The content of the Dockerfile to convert"),
			),
			mcp.WithString("organization",
				mcp.Description("The Chainguard organization to use (defaults to 'ORG')"),
			),
			mcp.WithString("registry",
				mcp.Description("Alternative registry to use instead of cgr.dev"),
			),
		),
		convertDockerfileHandler,
	)

	// Add a Dockerfile file analyzer tool
	s.AddTool(
		mcp.NewTool("analyze_dockerfile",
			mcp.WithDescription("Analyze a Dockerfile and provide information about its structure"),
			mcp.WithString("dockerfile_content",
				mcp.Required(),
				mcp.Description("The content of the Dockerfile to analyze"),
			),
		),
		analyzeDockerfileHandler,
	)

	// Add a resource for built-in Chainguard image mappings
	s.AddResource(
		mcp.NewResource(
			"chainguard://image-mappings",
			"Chainguard Image Mappings",
			mcp.WithResourceDescription("List of available Chainguard images and their mappings"),
			mcp.WithMIMEType("application/json"),
		),
		getChainGuardMappingsHandler,
	)

	// Start the server
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

// convertDockerfileHandler handles the convert_dockerfile tool requests
func convertDockerfileHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	dockerfileContent, ok := request.Params.Arguments["dockerfile_content"].(string)
	if !ok || dockerfileContent == "" {
		return mcp.NewToolResultError("Dockerfile content cannot be empty"), nil
	}

	// Extract optional parameters with defaults
	organization := "ORG"
	if org, ok := request.Params.Arguments["organization"].(string); ok && org != "" {
		organization = org
	}

	var registry string
	if reg, ok := request.Params.Arguments["registry"].(string); ok {
		registry = reg
	}

	// Convert the Dockerfile
	converted, err := convertDockerfile(ctx, dockerfileContent, organization, registry)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error converting Dockerfile: %v", err)), nil
	}

	// Return the result
	return mcp.NewToolResultText(converted), nil
}

// analyzeDockerfileHandler handles the analyze_dockerfile tool requests
func analyzeDockerfileHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	dockerfileContent, ok := request.Params.Arguments["dockerfile_content"].(string)
	if !ok || dockerfileContent == "" {
		return mcp.NewToolResultError("Dockerfile content cannot be empty"), nil
	}

	// Parse the Dockerfile
	dockerfile, err := dfc.ParseDockerfile(ctx, []byte(dockerfileContent))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse Dockerfile: %v", err)), nil
	}

	// Count stages and collect base images
	stageCount := 0
	baseImages := []string{}
	packageManagers := map[string]bool{}

	for _, line := range dockerfile.Lines {
		if line.From != nil {
			stageCount++
			baseImages = append(baseImages, line.From.Orig)
		}
		if line.Run != nil && line.Run.Manager != "" {
			packageManagers[string(line.Run.Manager)] = true
		}
	}

	// Build package manager list
	packageManagerList := []string{}
	for pm := range packageManagers {
		packageManagerList = append(packageManagerList, pm)
	}

	// Build analysis text
	analysis := fmt.Sprintf("Dockerfile Analysis:\n\n")
	analysis += fmt.Sprintf("- Total stages: %d\n", stageCount)
	analysis += fmt.Sprintf("- Base images: %s\n", strings.Join(baseImages, ", "))
	analysis += fmt.Sprintf("- Package managers: %s\n", strings.Join(packageManagerList, ", "))

	// Return the result
	return mcp.NewToolResultText(analysis), nil
}

// getChainGuardMappingsHandler provides information about available Chainguard images
func getChainGuardMappingsHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Build a JSON string with some of the mappings
	// In a real implementation, this would be more comprehensive
	mappingsJSON := `{
		"images": {
			"node": "node",
			"nodejs": "node",
			"python": "python",
			"golang": "go",
			"go": "go",
			"java": "jdk",
			"openjdk": "jdk",
			"nginx": "nginx",
			"ubuntu": "chainguard-base",
			"debian": "chainguard-base",
			"alpine": "chainguard-base",
			"busybox": "chainguard-base",
			"php": "php",
			"ruby": "ruby"
		}
	}`

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "chainguard://image-mappings",
			MIMEType: "application/json",
			Text:     mappingsJSON,
		},
	}, nil
}

// convertDockerfile converts a Dockerfile to use Chainguard Images and APKs
func convertDockerfile(ctx context.Context, dockerfileContent, organization, registry string) (string, error) {
	// Parse the Dockerfile
	dockerfile, err := dfc.ParseDockerfile(ctx, []byte(dockerfileContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse Dockerfile: %w", err)
	}

	// Create options for conversion
	opts := dfc.Options{
		Organization: organization,
	}

	// If registry is provided, set it in options
	if registry != "" {
		opts.Registry = registry
	}

	// Convert the Dockerfile
	converted, err := dockerfile.Convert(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("failed to convert Dockerfile: %w", err)
	}

	// Return the converted Dockerfile as a string
	return converted.String(), nil
}
