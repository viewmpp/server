FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN git config --global --add safe.directory /app

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /api ./cmd/web

FROM alpine:3.24

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -S -u 10001 -H -D server

COPY --from=builder /api /api

USER server

EXPOSE 4000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:4000/api/v1/healthcheck >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/api"]
