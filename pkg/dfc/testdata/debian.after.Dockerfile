FROM cgr.dev/myorg/debian:latest-dev
USER root
RUN apk add -U curl nginx
CMD ["nginx", "-g", "daemon off;"] 