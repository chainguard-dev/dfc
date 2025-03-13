FROM cgr.dev/ORGANIZATION/alpine:latest

# First, update package repository and install dependencies
RUN apk add -U build-essential curl git nodejs npm

# Install some more packages
RUN apk add -U python3 py3-pip

# Set up the application
WORKDIR /app
COPY . .

# Install application dependencies
RUN npm install && \
 npm run build

CMD ["npm", "start"] 