FROM debian:11
RUN apt-get update && apt-get install -y nginx curl
CMD ["nginx", "-g", "daemon off;"] 