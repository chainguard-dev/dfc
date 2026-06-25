FROM golang:1.21

RUN apt-get update && apt-get install -y \
    git \
    ca-certificates \
    curl

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

EXPOSE 8080
CMD ["./main"]