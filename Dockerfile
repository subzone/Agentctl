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

# Runtime stage: Alpine base image - MUCH smaller and practical for CLI tools
FROM alpine:3.20 AS runtime

# Copy the binary
COPY --from=build /out/m /usr/local/bin/m

# Copy examples for demo purposes
COPY --from=build /src/examples /examples

# Set working directory
WORKDIR /work

# Alpine doesn't have nonroot user, but we can create one if needed
# For CLI tools, root is fine as long as you're not running untrusted code
ENTRYPOINT ["/usr/local/bin/m"]