# Based on patterns from https://gist.github.com/BretFisher/da34530726ff8076b83b583e527e91ed
# This Dockerfile demonstrates a Node.js app with Ubuntu and apt-get packages

FROM ubuntu:20.04

# Set environment variables
ENV DEBIAN_FRONTEND=noninteractive
ENV NODE_VERSION=16.x

# Update and install dependencies
RUN apt-get update && apt-get upgrade -y && \
    apt-get install -y curl wget gnupg git build-essential python3 && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Add Node.js repository and install
RUN curl -sL https://deb.nodesource.com/setup_${NODE_VERSION} | bash - && \
    apt-get install -y nodejs && \
    npm install -g npm@latest

# Add a non-root user
RUN useradd -m -s /bin/bash appuser
WORKDIR /home/appuser/app
RUN chown -R appuser:appuser /home/appuser

# Switch to non-root user
USER appuser

# Copy application files
COPY --chown=appuser:appuser package*.json ./
RUN npm install

# Copy the rest of the application
COPY --chown=appuser:appuser . .

# Expose port
EXPOSE 3000

# Start command
CMD ["npm", "start"] 