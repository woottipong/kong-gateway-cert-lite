# syntax=docker/dockerfile:1
# check=error=true

FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/kong-cert-lite ./cmd/kong-cert-lite
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck
RUN mkdir -p /out/data

FROM gcr.io/distroless/base-debian12:nonroot AS runtime

LABEL org.opencontainers.image.title="Kong CertOps" \
      org.opencontainers.image.description="Internal TLS certificate lifecycle manager for Kong Gateway OSS" \
      org.opencontainers.image.source="kong-cert-lite"

COPY --from=builder --link /out/kong-cert-lite /kong-cert-lite
COPY --from=builder --link /out/healthcheck /healthcheck
COPY --from=builder --chown=nonroot:nonroot --link /out/data /data

ENV APP_ADDR=:8080 \
    APP_DB_PATH=/data/app.db \
    APP_CERT_DIR=/data/certs \
    APP_ACCOUNT_DIR=/data/accounts \
    LETSENCRYPT_ENV=staging \
    AUTO_RENEW_CRON="0 3 * * *"

USER nonroot:nonroot
VOLUME ["/data"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/healthcheck", "http://127.0.0.1:8080/healthz"]

ENTRYPOINT ["/kong-cert-lite"]
