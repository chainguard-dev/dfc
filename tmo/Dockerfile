FROM python:3.12-slim-bullseye AS python-dependencies-base

RUN apt-get update \
# Make sure to update the _Python Dependency Management_ section of the root
# README file in this repo if you change the pipenv version number
&& pip3 install --no-cache-dir pipenv==2024.0.1 \
# deps for psycopg
&& apt-get install -qy libpq-dev gcc