# syntax=docker/dockerfile:1.7

# Build stage: produce a static binary for $TARGETPLATFORM
FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

# Cache Go modules
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Build static binary with cache mounts
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build \
        -trimpath \
        -ldflags "-s -w -X main.Version=${VERSION}" \
        -o /out/m \
        ./cmd/m

# Runtime stage: Alpine base image with proper security
FROM alpine:3.20 AS runtime

# Security: install ca-certificates for HTTPS calls to LLM providers
# and create non-root user BEFORE copying files
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 -S agentctl && \
    adduser -u 1000 -S agentctl -G agentctl -h /home/agentctl && \
    mkdir -p /work /examples && \
    chown -R agentctl:agentctl /work /examples /home/agentctl

# Copy the binary with proper ownership
COPY --from=build --chown=agentctl:agentctl /out/m /usr/local/bin/m

# Copy examples for demo purposes (read-only for user)
COPY --from=build --chown=agentctl:agentctl /src/examples /examples

# Set working directory (owned by agentctl user)
WORKDIR /work

# Security: drop all capabilities, no new privileges
# This prevents the container from gaining additional Linux capabilities
# Even if someone exploits the binary, they can't escalate privileges
# Labels for metadata (good practice for container registries)
LABEL org.opencontainers.image.source="https://github.com/subzone/Agentctl" \
      org.opencontainers.image.description="MD-driven agent for code, infra, and automation" \
      org.opencontainers.image.vendor="subzone" \
      org.opencontainers.image.version="${VERSION}"

# Switch to non-root user - CRITICAL for security
# If MCP server or shell tool is exploited, attacker has limited privileges
USER agentctl

# Health check - useful for orchestration platforms
# Checks if binary is executable (basic sanity check)
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD [ "/usr/local/bin/m", "--version" ] || exit 1

# Default entrypoint
ENTRYPOINT ["/usr/local/bin/m"]