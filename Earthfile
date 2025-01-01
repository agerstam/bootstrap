VERSION 0.8
FROM golang:1.15-alpine3.13
WORKDIR /go-workdir

build:
    COPY go.mod go.sum .
    #COPY cmd/udm/main.go .
    COPY . ./
    RUN go mod download
    RUN go build -o output/bootstrap ./cmd/udm/main.go
    SAVE ARTIFACT output/bootstrap AS LOCAL output/bootstrap

docker:
    COPY +build/bootstrap .
    ENTRYPOINT ["/go-workdir/bootstrap"]
    SAVE IMAGE udm-bootstrap:latest
