FROM cgr.dev/chainguard/alpine:latest

RUN apk add -U curl

CMD ["echo", "hello"]

# This is a trailing comment
# This is another trailing comment
