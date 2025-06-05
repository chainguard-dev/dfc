/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/chainguard-dev/dfc/pkg/dfc"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Version information
const (
	Version = "dev"
)

func main() {
	// Set up logging to stderr for diagnostics
	logger := log.New(os.Stderr, "[dfc-mcp] ", log.LstdFlags)
	logger.Printf("Starting dfc MCP Server v%s", Version)

	// Create a context that listens for termination signals
	_, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create an MCP server instance
	s := server.NewMCPServer(
		"dfc - Dockerfile Converter",
		Version,
		server.WithLogging(),
		server.WithRecovery(),
		server.WithToolCapabilities(true),
	)

	// Define the Dockerfile converter tool
	dockerfileConverterTool := mcp.NewTool("convert_dockerfile",
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
	)

	// Add a healthcheck tool for diagnostics
	healthcheckTool := mcp.NewTool("healthcheck",
		mcp.WithDescription("Check if the dfc MCP server is running correctly"),
	)

	// Add the handler for the Dockerfile converter tool
	s.AddTool(dockerfileConverterTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger.Printf("Received convert_dockerfile request")

		// Extract parameters
		dockerfileContent, ok := request.Params.Arguments["dockerfile_content"].(string)
		if !ok || dockerfileContent == "" {
			logger.Printf("Error: Empty dockerfile content in request")
			return mcp.NewToolResultError("Dockerfile content cannot be empty"), nil
		}

		// Log a sample of the Dockerfile content (first 50 chars)
		contentPreview := dockerfileContent
		if len(contentPreview) > 50 {
			contentPreview = contentPreview[:50] + "..."
		}
		logger.Printf("Processing Dockerfile (preview): %s", contentPreview)

		// Extract optional parameters with defaults
		organization := "ORG"
		if org, ok := request.Params.Arguments["organization"].(string); ok && org != "" {
			organization = org
			logger.Printf("Using custom organization: %s", organization)
		}

		var registry string
		if reg, ok := request.Params.Arguments["registry"].(string); ok && reg != "" {
			registry = reg
			logger.Printf("Using custom registry: %s", registry)
		}

		// Convert the Dockerfile
		convertedDockerfile, err := convertDockerfile(ctx, dockerfileContent, organization, registry)
		if err != nil {
			logger.Printf("Error converting Dockerfile: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("Error converting Dockerfile: %v", err)), nil
		}

		// Log success
		logger.Printf("Successfully converted Dockerfile (length: %d bytes)", len(convertedDockerfile))

		// Return the result
		return mcp.NewToolResultText(convertedDockerfile), nil
	})

	// Add the healthcheck handler
	s.AddTool(healthcheckTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger.Printf("Received healthcheck request")

		// Create test Dockerfile content
		testDockerfile := "FROM alpine\nRUN apk add --no-cache curl"

		// Try a test conversion to ensure dfc package is working
		_, err := convertDockerfile(ctx, testDockerfile, "ORG", "")
		if err != nil {
			logger.Printf("Healthcheck failed: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("Healthcheck failed: %v", err)), nil
		}

		// If we get here, all systems are operational
		statusInfo := map[string]interface{}{
			"status":      "ok",
			"version":     Version,
			"dfc_package": "operational",
		}

		statusJSON, _ := json.Marshal(statusInfo)
		return mcp.NewToolResultText(fmt.Sprintf("Healthcheck passed: %s", string(statusJSON))), nil
	})

	// Add a tool that analyzes a Dockerfile
	analyzeDockerfileTool := mcp.NewTool("analyze_dockerfile",
		mcp.WithDescription("Analyze a Dockerfile and provide information about its structure"),
		mcp.WithString("dockerfile_content",
			mcp.Required(),
			mcp.Description("The content of the Dockerfile to analyze"),
		),
	)

	// Add the analyzer handler
	s.AddTool(analyzeDockerfileTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger.Printf("Received analyze_dockerfile request")

		// Extract parameters
		dockerfileContent, ok := request.Params.Arguments["dockerfile_content"].(string)
		if !ok || dockerfileContent == "" {
			logger.Printf("Error: Empty dockerfile content in analyze request")
			return mcp.NewToolResultError("Dockerfile content cannot be empty"), nil
		}

		// Parse the Dockerfile
		dockerfile, err := dfc.ParseDockerfile(ctx, []byte(dockerfileContent))
		if err != nil {
			logger.Printf("Error parsing Dockerfile for analysis: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to parse Dockerfile: %v", err)), nil
		}

		// Analyze the Dockerfile
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

		// Build package manager list
		// TODO: something seems to be off here, returning "No package managers detected"
		packageManagerList := []string{}
		for pm := range packageManagers {
			packageManagerList = append(packageManagerList, pm)
		}

		// Build analysis text
		analysis := "Dockerfile Analysis:\n\n"
		analysis += fmt.Sprintf("- Total stages: %d\n", stageCount)
		analysis += fmt.Sprintf("- Base images: %s\n", strings.Join(baseImages, ", "))
		if len(packageManagerList) > 0 {
			analysis += fmt.Sprintf("- Package managers: %s\n", strings.Join(packageManagerList, ", "))
		} else {
			analysis += "- No package managers detected\n"
		}

		logger.Printf("Successfully analyzed Dockerfile: %d stages, %d base images",
			stageCount, len(baseImages))

		// Return the result
		return mcp.NewToolResultText(analysis), nil
	})

	// Announce that we're ready to serve
	logger.Printf("MCP server initialization complete, ready to handle requests")

	// Start the server
	if err := server.ServeStdio(s); err != nil {
		logger.Printf("Server error: %v", err)
		os.Exit(1)
	}
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
