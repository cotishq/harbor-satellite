# ── Prebuilt path ────────────────────────────────────────────────────────────
# Used when a cross-compiled binary is supplied via the build context.
# Only installs ca-certificates; no Go toolchain, no module download, no
# source copy. The binary is copied in from the build-context path set by
# the caller (publish-platform-prebuilt task).
FROM alpine:3.20 AS builder-prebuilt

RUN apk add --no-cache ca-certificates

ARG PREBUILT_BINARY=""
COPY ${PREBUILT_BINARY} /app-bin

# ── From-source path ─────────────────────────────────────────────────────────
# Full in-Docker Go build. Git is required by go mod download for
# VCS-stamped dependencies.
FROM golang:1.26.5-alpine AS builder-source

WORKDIR /app

RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary. Default: no extra tags (production-equivalent of main).
# Pass --build-arg GO_TAGS=parsec to opt into the PARSEC code path.
ARG GO_TAGS=""
ARG COMPONENT=satellite
RUN CGO_ENABLED=0 GOOS=linux go build -tags "${GO_TAGS}" -o /app-bin ./cmd/${COMPONENT}

# ── Builder selector ──────────────────────────────────────────────────────────
# BuildKit evaluates ARGs in FROM directives, allowing the caller to choose
# the builder stage at build time:
#
#   Prebuilt:    docker buildx build --build-arg BUILDER_MODE=prebuilt
#                                    --build-arg PREBUILT_BINARY=./prebuilt-binary ...
#   From source: docker buildx build ...   (default; no extra args required)
#
# The runtime stage always COPYs /app-bin from whichever builder was selected.
ARG BUILDER_MODE=source
FROM builder-${BUILDER_MODE} AS builder

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

# Copy binary and Ground Control migrations from builder. The migrations copy is
# harmless for the satellite image and keeps one Dockerfile for both binaries.
COPY --from=builder /app-bin /app/app
COPY --from=builder-source /app/internal/groundcontrol/sql/schema /migrations

# Create data directory
RUN mkdir -p /data

# Create non-root user
RUN adduser -D -g '' appuser
RUN chown -R appuser:appuser /data
USER appuser

WORKDIR /data

EXPOSE 8080

ENTRYPOINT ["/app/app"]
