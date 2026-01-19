# Podman on WSL

This directory contains examples and instructions for running containers using **Podman** in a WSL (Windows Subsystem for Linux) environment.

## Why Podman?
Podman is a daemonless, open-source, Linux native tool for finding, building, managing, and trailing containers and pods. It is a great alternative to Docker, especially in environments where you want to avoid a background daemon or root privileges.

## Getting Started

### 1. Installation
If Podman is not installed, run:
```bash
sudo apt update
sudo apt install -y podman
```

### 2. Basic Commands
Podman's CLI is intentionally compatible with Docker. You can often just alias `alias docker=podman`.

- **Build an image**: `podman build -t my-image .`
- **Run a container**: `podman run --rm my-image`
- **List containers**: `podman ps`
- **List images**: `podman images`

## Go Example (Scratch-based)
The files in this directory demonstrate a minimal Go application running in a `scratch` container.

### Building and Running
```bash
# Build the image
podman build -t go-scratch-example .

# Run it
podman run --rm go-scratch-example
```

### Tips for Podman on WSL
- **Registry Resolution**: Podman requires fully qualified image names. Instead of `FROM golang`, use `FROM docker.io/library/golang`.
- **Rootless by default**: Podman runs containers in rootless mode by default, which is more secure.
- **No Daemon**: There is no `systemd` requirement for basic container operations, making it very lightweight for WSL.
