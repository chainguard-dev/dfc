FROM cgr.dev/ORGANIZATION/alpine:latest-dev
USER root
RUN apk add -U py3-pip python3
WORKDIR /app
COPY . .
CMD ["python3", "app.py"] 