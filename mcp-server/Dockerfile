FROM cgr.dev/chainguard/go:latest@sha256:8df9dd7beb988f8f912df54c036ea44e4b11455da5d33e2a4eb2d19de75820c8 AS builder

WORKDIR /app

COPY go.mod ./
COPY main.go ./

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o mcp-server

FROM cgr.dev/chainguard/static:latest@sha256:9cef3c6a78264cb7e25923bf1bf7f39476dccbcc993af9f4ffeb191b77a7951e

WORKDIR /app
COPY --from=builder /app/mcp-server /app/mcp-server

ENTRYPOINT ["/app/mcp-server"] 