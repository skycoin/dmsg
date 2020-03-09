FROM golang:1.13-alpine AS builder
ARG CGO_ENABLED=0

ENV CGO_ENABLED=${CGO_ENABLED} \
	GOOS=linux  \
	GOARCH=amd64 \
	GO111MODULE=on

COPY . /dmsg

WORKDIR /dmsg

# Build dmsg discovery
RUN go build -o /release/dmsg-discovery ./cmd/dmsg-discovery

# Build image
FROM scratch

COPY --from=builder /release/dmsg-discovery /usr/local/bin/dmsg-discovery
ENTRYPOINT [ "dmsg-discovery" ]
