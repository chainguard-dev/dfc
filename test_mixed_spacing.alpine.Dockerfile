FROM cgr.dev/chainguard/alpine:latest AS base

# comment with blank line after

ARG ABC

# comment without blank line
CMD ["echo", "hello"]
