VERSION 0.8

# Builder target: Build the Go binary
builder:
    FROM golang:1.23.4
    WORKDIR /go-workdir
    COPY go.mod go.sum .                 # Copy Go module files
    COPY . ./                            # Copy the entire project
    RUN go mod download                  # Download dependencies
    RUN go build -ldflags="-s -w" -o bootstrap ./cmd/udm/main.go # Build the binary
    SAVE ARTIFACT bootstrap AS LOCAL output/bootstrap            # Save locally

# Docker target: Create a smaller runtime image
docker:
    FROM alpine:latest                   # Use a minimal Alpine image for the runtime
    RUN apk add --no-cache cryptsetup bash # Install cryptsetup and bash
    COPY +builder/bootstrap /bootstrap   # Copy the built binary from the builder target
    COPY scripts/config.yml /config.yml  # Copy the configuration file
    ENTRYPOINT ["/bootstrap"]            # Set the binary as the entry point
    SAVE IMAGE udm-bootstrap:latest      # Save the Docker image
