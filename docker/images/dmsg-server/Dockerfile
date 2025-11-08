FROM golang:1.25-alpine AS builder
ARG CGO_ENABLED=0

ENV CGO_ENABLED=${CGO_ENABLED} \
	GOOS=linux  \
	GOARCH=amd64 \
	GO111MODULE=on

COPY . /dmsg
WORKDIR /dmsg

# Build dmsg binary from repo root
RUN go build -tags netgo -mod=vendor "-ldflags=-w -s" -o /release/dmsg .

# Build image
FROM alpine:latest

COPY --from=builder /release/dmsg /usr/local/bin/dmsg

RUN mkdir -p /e2e && \
    echo '{\
  "public_key": "03b88c1335c28264c5e40ffad67eee75c2f2c39bda27015d6e14a0e90eaa78a41c",\
  "secret_key": "c46cf86c2e13e2ae41a9014d0e3b19e1b1dc1ea5c3e18aee4adf3c4db84ddca7",\
  "discovery": "http://dmsg-discovery:9090",\
  "local_address": "dmsg-server:8080",\
  "log_level": "info"\
}' > /e2e/dmsg-server.json

STOPSIGNAL SIGINT

ENTRYPOINT [ "dmsg", "server" ]
