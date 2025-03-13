FROM ubuntu:22.04

# First, update package repository and install dependencies
RUN apt-get update -qq && \
    apt-get install -y --no-install-recommends \
    build-essential \
    curl \
    git \
    nodejs \
    npm && \
    # Clean up apt cache
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install some more packages
RUN apt-get update && apt-get install -y python3 python3-pip

# Set up the application
WORKDIR /app
COPY . .

# Install application dependencies
RUN npm install && \
    apt-get update && \
    apt-get install -y redis-tools && \
    npm run build

CMD ["npm", "start"] 