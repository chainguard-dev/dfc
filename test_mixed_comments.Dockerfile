FROM ubuntu:22.04

# This is a comment before install
# Second line of comment

RUN apt-get update && apt-get install -y curl

# Comment before CMD
# Multiple lines

CMD ["echo", "hello"]

# This is a trailing comment
# This is another trailing comment
