FROM alpine:3.14
RUN apk add --no-cache python3 py3-pip
WORKDIR /app
COPY . .
CMD ["python3", "app.py"] 