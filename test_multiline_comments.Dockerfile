FROM ubuntu:22.04

# This is a comment
# This is a second line of the comment
# This is a third line of the comment

COPY . .

CMD ["echo", "hello"]
