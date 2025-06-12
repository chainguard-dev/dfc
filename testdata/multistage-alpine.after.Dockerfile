FROM cgr.dev/ORG/chainguard-base:latest AS builder
USER root

RUN apk add --no-cache git nodejs npm

FROM cgr.dev/ORG/chainguard-base:latest

RUN apk add --no-cache nodejs npm

COPY --from=builder package.json package-lock.json ./
RUN npm ci --only=production

COPY src/ ./src/
COPY public/ ./public/

EXPOSE 3000
CMD ["node", "src/index.js"]