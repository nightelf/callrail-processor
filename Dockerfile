# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
WORKDIR /src

# Download modules first so this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

# Cross-compile for the target platform (e.g. linux/amd64 for most clusters,
# even when building on an Apple Silicon Mac). CGO off → fully static binary.
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/callprocessor .

# ---- Runtime stage ----
# Distroless: no shell or package manager, runs as a non-root user (UID 65532).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/callprocessor /callprocessor

ENV PORT=8080
EXPOSE 8080
# Numeric UID so Kubernetes runAsNonRoot can verify it (65532 = distroless "nonroot").
USER 65532:65532
ENTRYPOINT ["/callprocessor"]
