FROM alpine:3.18

RUN apk add --no-cache \
    nodejs \
    npm \
    git

COPY package.json package-lock.json ./
RUN npm ci --only=production

COPY src/ ./src/
COPY public/ ./public/

EXPOSE 3000
CMD ["node", "src/index.js"]