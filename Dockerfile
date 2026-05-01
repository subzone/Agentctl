# syntax=docker/dockerfile:1.7

# Build stage: produce a static binary for $TARGETPLATFORM. Pinned to the
# exact Go toolchain in go.mod so a reproducible image doesn't depend on
# whatever floats at `golang:1.26-alpine` later.
FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

# Module download is its own RUN so changes to source don't bust the
# module cache.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Build-cache mounts shave seconds off rebuilds; output is a static binary
# placed at /out/m. Cross-compile via TARGETOS/TARGETARCH so buildx
# can produce amd64 + arm64 artifacts from a single source tree.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build \
        -trimpath \
        -ldflags "-s -w -X main.Version=${VERSION}" \
        -o /out/m \
        ./cmd/m

# Runtime stage: distroless static keeps the image to ~12-15 MB and ships
# no shell/package manager, which matches the project's "small footprint"
# goal in PLAN.md. The :nonroot variant runs as UID 65532.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/m /m

# Bundle the example agent docs at /examples so users can demo the image
# without mounting anything: `docker run … run /examples/agents/hello.md "hi"`.
COPY --from=build /src/examples /examples

WORKDIR /work
USER nonroot:nonroot

ENTRYPOINT ["/m"]
