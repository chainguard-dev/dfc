# Originally from https://github.com/chainguard-dev/dfc/issues/90
FROM --platform=linux/amd64 cgr.dev/ORG/go:1.23-dev AS build
USER root
RUN apk add --no-cache make
WORKDIR /usr/src/app
COPY . .
FROM --platform=linux/amd64 cgr.dev/ORG/chainguard-base:latest
WORKDIR /usr/src/app
EXPOSE 8880 8881 8882
CMD ["./init.sh"]
