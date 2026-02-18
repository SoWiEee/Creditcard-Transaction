FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed


FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates netcat-openbsd

COPY --from=builder /out/server /app/server
COPY --from=builder /out/seed   /app/seed

COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

EXPOSE 3000

ENV PORT=3000 \
    RUN_SEED=0 \
    WAIT_FOR_DEPS=1 \
    DB_WAIT_TIMEOUT=30 \
    REDIS_WAIT_TIMEOUT=10

ENTRYPOINT ["/app/entrypoint.sh"]