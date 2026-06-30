# Multi-stage Dockerfile for BidWriter Go services.
#
# Build context: repository root (so we can read go.work and shared pkg).
# Build args:
#   SERVICE_PATH   e.g. services/api-gateway
#   SERVICE_NAME   binary name (defaults to last path component of SERVICE_PATH)
#
# Example:
#   docker build -f docker/Dockerfile \
#     --build-arg SERVICE_PATH=services/api-gateway \
#     -t bidwriter/api-gateway .

ARG GO_VERSION=1.23

FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /src

# Install build deps for cgo (none required currently but keep for future)
RUN apk add --no-cache git ca-certificates

# Copy go.work first so layer caches when service code changes
COPY go.work go.work.sum* ./
COPY shared/ ./shared/
COPY services/ ./services/

ARG SERVICE_PATH=services/api-gateway
ARG SERVICE_NAME=api-gateway

WORKDIR /src/${SERVICE_PATH}

# Cache deps
RUN go mod download

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/${SERVICE_NAME} ./cmd/${SERVICE_NAME}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget
RUN addgroup -S bidwriter && adduser -S -G bidwriter bidwriter

ARG SERVICE_NAME=api-gateway
ENV SERVICE_NAME=${SERVICE_NAME}

WORKDIR /app
COPY --from=builder /out/${SERVICE_NAME} /app/${SERVICE_NAME}

USER bidwriter
EXPOSE 8080

# Healthcheck (all services expose /healthz)
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/${SERVICE_NAME}"]