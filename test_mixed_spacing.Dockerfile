FROM node:20.15.0 AS base

# comment with blank line after

ARG ABC

# comment without blank line
CMD ["echo", "hello"]
