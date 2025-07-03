# This is to test that "USER root" is added appropriately
# even though there is a RUN command with "useradd" in it
FROM cgr.dev/ORG/php:8.3-dev
USER root

RUN apk add --no-cache curl git libxml2-dev unzip zip

# Install Composer and set up application
COPY --from=composer:latest /usr/bin/composer /usr/bin/composer

WORKDIR /app
COPY . /app

# set up nonroot system user
RUN adduser --system --shell /bin/bash nonroot && \
    chown -R nonroot /app && \
    cd /app && \
    composer install

USER nonroot
ENTRYPOINT [ "php", "minicli", "mycommand" ]
