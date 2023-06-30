// package main cmd/dmsgproxy/dmsgproxy.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/armon/go-socks5"
)

type customResolver struct{}

func (r *customResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	// Handle custom name resolution for .dmsg domains
	if strings.HasSuffix(name, ".dmsg") {
		// Resolve .dmsg domains to the desired IP address
		ip := net.ParseIP("127.0.0.1") // Replace with your desired IP address
		if ip == nil {
			return ctx, nil, fmt.Errorf("failed to parse IP address for .dmsg domain")
		}
		return ctx, ip, nil
	}

	// Use default name resolution for other domains
	return ctx, nil, nil
}

func main() {
	// Create a SOCKS5 server with custom name resolution
	conf := &socks5.Config{
		Resolver: &customResolver{},
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			fmt.Println(addr)
			return net.Dial(network, addr)
		},
	}

	// Start the SOCKS5 server
	addr := "127.0.0.1:4445"
	log.Printf("SOCKS5 proxy server started on %s", addr)

	server, err := socks5.New(conf)
	if err != nil {
		log.Fatalf("failed to create SOCKS5 server: %v", err)
	}

	err = server.ListenAndServe("tcp", addr)
	if err != nil {
		log.Fatalf("failed to start SOCKS5 server: %v", err)
	}
}
