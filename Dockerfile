# ---- Build Stage ----
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /out/cc-switch .

# ---- Runtime Stage ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -S ccswitch && \
    adduser -S -G ccswitch ccswitch

# Binary
COPY --from=builder /out/cc-switch /usr/local/bin/cc-switch

# Web static files
COPY web/ /opt/cc-switch-server/web/

# Data volume
RUN mkdir -p /var/lib/cc-switch-server/backups && \
    chown -R ccswitch:ccswitch /var/lib/cc-switch-server

WORKDIR /opt/cc-switch-server
USER ccswitch

EXPOSE 9876

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://127.0.0.1:9876/api/proxy/status || exit 1

ENTRYPOINT ["cc-switch"]
CMD ["serve", "--host", "0.0.0.0", "--port", "9876", "--data-dir", "/var/lib/cc-switch-server"]
