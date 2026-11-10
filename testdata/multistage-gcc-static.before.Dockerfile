FROM ubuntu:20.04

RUN apt-get update && apt-get install -y \
    gcc \
    libc6-dev \
    make \
    curl \
    git

COPY hello.c /app/hello.c
COPY Makefile /app/Makefile

WORKDIR /app
RUN gcc -static -o hello hello.c

EXPOSE 8080
CMD ["./hello"]