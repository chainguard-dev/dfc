# This is to test that "USER root" is added appropriately
# even though there is a RUN command with "useradd" in it
FROM php:8.3-cli

RUN apt-get update && apt-get install -y \
    git \
    curl \
    libxml2-dev \
    zip \
    unzip

# Install Composer and set up application
COPY --from=composer:latest /usr/bin/composer /usr/bin/composer

WORKDIR /app
COPY . /app

# set up nonroot system user
RUN useradd -r -s /bin/bash nonroot && \
    chown -R nonroot /app && \
    cd /app && composer install

USER nonroot
ENTRYPOINT [ "php", "minicli", "mycommand" ]
