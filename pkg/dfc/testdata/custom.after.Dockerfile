FROM cgr.dev/custom-org/ubuntu:latest-dev
USER root
RUN apk add -U nodejs npm py3-pip python3
WORKDIR /app
COPY . .
CMD ["python3", "app.py"] 