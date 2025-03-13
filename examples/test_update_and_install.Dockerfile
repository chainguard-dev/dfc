FROM ubuntu:22.04

RUN apt-get update -qq && apt-get install -y nano zsh && \
  chmod +x bin/oh-my-zsh.sh && \
  sh -c "RUNZSH=no bin/oh-my-zsh.sh"

CMD ["zsh"] 