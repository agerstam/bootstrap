VERSION 0.8
FROM golang:1.15-alpine3.13
WORKDIR /go-workdir

build:
    COPY go.mod go.sum .            # Copy Go module files
    COPY . ./                       # Copy the entire project
    RUN go mod download             # Download dependencies
    RUN go build -o output/bootstrap ./cmd/udm/main.go # Build the binary
    SAVE ARTIFACT output/bootstrap AS LOCAL output/bootstrap # Save locally

docker:
    RUN apk add --no-cache cryptsetup bash # Install cryptsetup and bash
    COPY output/bootstrap .         # Copy the built binary to the container
    COPY scripts/config.yml ./      # Copy the configuration file to the container
    ENTRYPOINT ["/go-workdir/bootstrap"] # Set the binary as the entry point
    SAVE IMAGE udm-bootstrap:latest # Save the Docker image
