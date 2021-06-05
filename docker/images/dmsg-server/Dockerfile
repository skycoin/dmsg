FROM golang:1.16-alpine AS builder
ARG CGO_ENABLED=0

ENV CGO_ENABLED=${CGO_ENABLED} \
	GOOS=linux  \
	GOARCH=amd64 \
	GO111MODULE=on

COPY . /dmsg
WORKDIR /dmsg

# Build dmsg server
RUN apk add --no-cache bash make git && \
    make build-deploy

# Build image
FROM alpine:latest

COPY --from=builder /release/dmsg-server /usr/local/bin/dmsg-server

STOPSIGNAL SIGINT

ENTRYPOINT [ "dmsg-server" ]
