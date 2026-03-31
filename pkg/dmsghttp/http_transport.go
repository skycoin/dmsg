// Package dmsghttp pkg/dmsghttp/http_transport.go
package dmsghttp

import (
	"context"
	"fmt"
	"net"
	"net/http"

	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
)

const defaultHTTPPort = uint16(80)

// HTTPTransport implements http.RoundTripper using dmsg streams as the
// underlying transport. It wraps Go's http.Transport with a custom DialContext
// so that standard HTTP connection pooling and keep-alive work over dmsg.
//
// Do not confuse this with a Skywire Transport implementation.
type HTTPTransport struct {
	ctx       context.Context
	dmsgC     *dmsg.Client
	transport *http.Transport
}

// MakeHTTPTransport makes an HTTPTransport.
func MakeHTTPTransport(ctx context.Context, dmsgC *dmsg.Client) HTTPTransport {
	t := HTTPTransport{
		ctx:   ctx,
		dmsgC: dmsgC,
	}
	t.transport = &http.Transport{
		DialContext: t.dialContext,
		// Connection pooling is disabled because dmsg streams use
		// noise-encrypted framing with per-stream handshakes. Reusing
		// streams across HTTP requests can cause EOF errors when the
		// server's ReadTimeout expires between requests, and POST
		// requests cannot be automatically retried.
		// The main latency savings come from route caching and
		// latency-sorted server selection in DialStream, not from
		// HTTP keep-alive.
		DisableKeepAlives: true,
	}
	// Close idle pooled connections when context is canceled so that
	// server-side goroutines can clean up without waiting for idle timeout.
	go func() {
		<-ctx.Done()
		t.transport.CloseIdleConnections()
	}()
	return t
}

// dialContext dials a dmsg stream for the given address.
// Called by http.Transport when it needs a new connection.
func (t HTTPTransport) dialContext(ctx context.Context, _, addr string) (net.Conn, error) {
	var hostAddr dmsg.Addr
	if err := hostAddr.Set(addr); err != nil {
		return nil, fmt.Errorf("invalid host address: %w", err)
	}
	if hostAddr.Port == 0 {
		hostAddr.Port = defaultHTTPPort
	}
	return t.dmsgC.DialStream(ctx, hostAddr)
}

// RoundTrip implements http.RoundTripper.
// It normalizes the URL scheme to "http" (so Go's transport handles it)
// and delegates to the pooled transport. The actual connection goes
// through dmsg via the custom DialContext.
func (t HTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Normalize scheme: callers may use "dmsg://" URLs.
	// Go's transport only understands "http" and "https".
	if req.URL.Scheme == "dmsg" {
		req = req.Clone(req.Context())
		req.URL.Scheme = "http"
	}
	return t.transport.RoundTrip(req)
}

// CloseIdleConnections closes any idle pooled connections.
func (t HTTPTransport) CloseIdleConnections() {
	t.transport.CloseIdleConnections()
}
