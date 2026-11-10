FROM cgr.dev/ORG/chainguard-base:latest AS builder
USER root

RUN apk add --no-cache curl git py3-pip python-3 python3-venv
FROM cgr.dev/ORG/chainguard-base:latest

COPY --from=builder requirements.txt /app/requirements.txt
COPY app.py /app/app.py
COPY static/ /app/static/

WORKDIR /app
RUN python3 -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"
RUN pip3 install -r requirements.txt

EXPOSE 8000
CMD ["python3", "app.py"]
