FROM debian:11

RUN apt-get update \
# comment line 1
# comment line 2
&& apt-get install -y nano vim && \
# another comment
echo bye
