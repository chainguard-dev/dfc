FROM cgr.dev/ORG/go:1.21-dev AS builder
USER root

RUN apk add --no-cache ca-certificates curl git
FROM cgr.dev/ORG/go:1.21-dev

COPY --from=builder go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

EXPOSE 8080
CMD ["./main"]