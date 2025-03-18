# From https://github.com/django/djangoproject.com/blob/main/Dockerfile

# pull official base image
FROM cgr.dev/ORG/python:3.12-dev
USER root

# set work directory
WORKDIR /usr/src/app

# set environment varibles
ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1

# install deb packages
RUN apk add -U gettext git libpq make rsync && \
    rm -rf /var/lib/apt/lists/*

ARG REQ_FILE=requirements/prod.txt

# install python dependencies
COPY ./requirements ./requirements
RUN apk add -U g++ gcc glibc-dev postgresql-dev zlib-dev && \
    python3 -m pip install --no-cache-dir -r ${REQ_FILE} && \
    rm -rf /var/lib/apt/lists/*

# copy project
COPY . .

# ENTRYPOINT is specified only in the local docker-compose.yml to avoid
# accidentally running it in deployed environments.
