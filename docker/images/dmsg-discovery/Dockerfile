FROM golang:1.16-alpine AS builder
ARG CGO_ENABLED=0

ENV CGO_ENABLED=${CGO_ENABLED} \
	GOOS=linux  \
	GOARCH=amd64 \
	GO111MODULE=on

COPY . /dmsg
WORKDIR /dmsg

# Build dmsg discovery
RUN apk add --no-cache make bash git && \
    make build-deploy

# Build image
FROM alpine:latest

COPY --from=builder /release/dmsg-discovery /usr/local/bin/dmsg-discovery
COPY --from=builder /dmsg/docker/images/dmsg-discovery/entrypoint.sh /usr/local/bin/entrypoint.sh

EXPOSE 9090
STOPSIGNAL SIGINT
ENTRYPOINT [ "sh", "-c", "entrypoint.sh" ]
