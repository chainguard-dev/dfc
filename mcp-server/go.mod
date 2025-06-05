module github.com/chainguard-dev/dfc/mcp-server

go 1.24

toolchain go1.24.2

require (
	github.com/chainguard-dev/dfc v0.0.0-20250101000000-000000000000
	github.com/mark3labs/mcp-go v0.25.0
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/chainguard-dev/clog v1.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/sys v0.26.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/chainguard-dev/dfc => ../
