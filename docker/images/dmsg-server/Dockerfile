FROM golang:1.13-alpine AS builder
ARG CGO_ENABLED=0

ENV CGO_ENABLED=${CGO_ENABLED} \
	GOOS=linux  \
	GOARCH=amd64 \
	GO111MODULE=on

COPY . /dmsg

WORKDIR /dmsg

# Build dmsg server
RUN go build -o /release/dmsg-server ./cmd/dmsg-server

# Build image
FROM scratch

COPY --from=builder /release/dmsg-server /usr/local/bin/dmsg-server
ENTRYPOINT [ "dmsg-server" ]
