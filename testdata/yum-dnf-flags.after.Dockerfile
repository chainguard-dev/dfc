# Make sure when the -y flag is used before the install keyword
# that conversion still occurs correctly
FROM cgr.dev/ORG/chainguard-base:latest
USER root

RUN apk add --no-cache apache2 php php-cli php-common

RUN apk add --no-cache apache2 php php-cli php-common

RUN apk add --no-cache apache2 php php-cli php-common
