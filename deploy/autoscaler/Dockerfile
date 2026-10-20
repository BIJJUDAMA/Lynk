# syntax=docker/dockerfile:1
FROM python:3.11-alpine

WORKDIR /app

RUN apk add --no-cache docker-cli docker-cli-compose

COPY controller.py /app/controller.py

ENV PYTHONUNBUFFERED=1
ENV COMPOSE_FILE=/workspace/docker-compose.yml

ENTRYPOINT ["python", "/app/controller.py"]
