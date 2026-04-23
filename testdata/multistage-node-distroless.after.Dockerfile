FROM cgr.dev/ORG/node:18-dev AS builder
USER root

RUN apk add --no-cache curl gcc git make python-3
FROM cgr.dev/ORG/node:18-dev

COPY --from=builder package*.json ./
RUN npm ci --only=production

COPY src/ ./src/
COPY public/ ./public/

EXPOSE 3000
CMD ["node", "src/index.js"]