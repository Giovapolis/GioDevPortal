# syntax=docker/dockerfile:1

# ---------- Build ----------
FROM golang:1.26-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/giodevportal ./cmd/server

# ---------- Runtime ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app -u 10001 app

WORKDIR /app
COPY --from=builder /out/giodevportal /app/giodevportal
COPY --from=builder /src/web /app/web

RUN mkdir -p /app/data && chown -R app:app /app
USER app

ENV SERVER_ADDR=:8080 \
    DB_PATH=/app/data/app.db \
    DATA_DIR=/app/data

EXPOSE 8080
VOLUME ["/app/data"]

ENTRYPOINT ["/app/giodevportal"]
