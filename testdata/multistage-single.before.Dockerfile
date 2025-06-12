FROM ubuntu:20.04

RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    curl \
    git

COPY requirements.txt /app/requirements.txt
COPY app.py /app/app.py
COPY static/ /app/static/

WORKDIR /app
RUN pip3 install -r requirements.txt

EXPOSE 8000
CMD ["python3", "app.py"]