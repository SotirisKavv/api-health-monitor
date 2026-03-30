# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/monitor-api ./cmd/monitor-api

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /data

WORKDIR /app

COPY --from=builder /bin/monitor-api /app/monitor-api

EXPOSE 8080

ENV ADDR=:8080
ENV DB_PATH=/data/monitor.db

ENTRYPOINT ["/app/monitor-api"]