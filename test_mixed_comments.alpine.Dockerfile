FROM cgr.dev/chainguard/alpine:latest

# This is a comment before install
# Second line of comment

RUN apk add -U curl

# Comment before CMD
# Multiple lines

CMD ["echo", "hello"]

# This is a trailing comment
# This is another trailing comment
