#!/bin/bash
set -e

# Build the server
go build -o mcp-server-go

# Run the server
./mcp-server-go 