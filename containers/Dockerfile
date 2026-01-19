# Use the official Golang alpine image as a builder stage
FROM docker.io/library/golang:1.23-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy the source code
COPY main.go .

# Build the Go application with static linking
# -ldflags="-s -w" reduces binary size
# CGO_ENABLED=0 ensures no dependency on C libraries (required for 'scratch')
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o hello-podman main.go

# Start from 'scratch' for the final minimal image
FROM scratch

# Copy the binary from the builder stage
COPY --from=builder /app/hello-podman /hello-podman

# Run the binary
ENTRYPOINT ["/hello-podman"]
