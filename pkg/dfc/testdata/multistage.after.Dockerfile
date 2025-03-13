FROM cgr.dev/ORGANIZATION/golang:latest-dev AS builder
USER root
WORKDIR /app
COPY . .
RUN go build -o app

FROM cgr.dev/ORGANIZATION/debian:latest-dev
USER root
RUN apk add -U ca-certificates
COPY --from=builder /app/app /usr/local/bin/
ENTRYPOINT ["app"] 