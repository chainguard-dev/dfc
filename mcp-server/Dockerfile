FROM cgr.dev/chainguard/go:latest AS builder

WORKDIR /app

COPY go.mod ./
COPY main.go ./

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o mcp-server

FROM cgr.dev/chainguard/static:latest

WORKDIR /app
COPY --from=builder /app/mcp-server /app/mcp-server

ENTRYPOINT ["/app/mcp-server"] 