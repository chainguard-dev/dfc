FROM ubuntu:22.04

RUN apt-get update && apt-get install -y curl

CMD ["echo", "hello"]

# This is a trailing comment
# This is another trailing comment
