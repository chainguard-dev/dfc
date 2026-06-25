FROM cgr.dev/ORG/chainguard-base:latest AS builder
USER root

RUN apk add --no-cache curl gcc git glibc-dev make
FROM cgr.dev/ORG/chainguard-base:latest

COPY --from=builder hello.c /app/hello.c
COPY Makefile /app/Makefile

WORKDIR /app
RUN gcc -static -o hello hello.c

EXPOSE 8080
CMD ["./hello"]