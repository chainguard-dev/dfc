FROM cgr.dev/ORGANIZATION/alpine:latest

RUN apk add -U nano zsh && \
  chmod +x bin/oh-my-zsh.sh && \
  sh -c "RUNZSH=no bin/oh-my-zsh.sh"

CMD ["zsh"] 