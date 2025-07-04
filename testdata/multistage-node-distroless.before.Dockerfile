FROM node:18

RUN apt-get update && apt-get install -y \
    python3 \
    make \
    g++ \
    git \
    curl

COPY package*.json ./
RUN npm ci --only=production

COPY src/ ./src/
COPY public/ ./public/

EXPOSE 3000
CMD ["node", "src/index.js"]