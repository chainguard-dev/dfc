# Originally from https://github.com/chainguard-dev/dfc/issues/90
FROM --platform=linux/amd64 golang:1.23.8-bookworm AS build
RUN apt-get update && apt-get install make -y
WORKDIR /usr/src/app
COPY . .
FROM --platform=linux/amd64 ubuntu:latest
WORKDIR /usr/src/app
EXPOSE 8880 8881 8882
CMD ["./init.sh"]
