# Dockerfile.builder
# Purpose: Pre-install cross-compilers and Go modules to speed up Dialtone ARM builds
FROM docker.io/library/golang:1.25.5

RUN apt-get update && apt-get install -y \
    gcc-aarch64-linux-gnu \
    gcc-arm-linux-gnueabihf \
    libc6-dev-arm64-cross \
    libc6-dev-armhf-cross \
    && rm -rf /var/lib/apt/lists/*

# Pre-download Go modules to bake them into the image
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# The dialtone build system can now use this image instead of installing on every run.
