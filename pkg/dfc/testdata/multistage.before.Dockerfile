FROM golang:1.18 AS builder
WORKDIR /app
COPY . .
RUN go build -o app

FROM debian:11
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=builder /app/app /usr/local/bin/
ENTRYPOINT ["app"] 