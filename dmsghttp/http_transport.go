package dmsghttp

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/skycoin/dmsg"
)

const defaultHTTPPort = uint16(80)

// HTTPTransport implements http.RoundTripper
// Do not confuse this with a Skywire Transport implementation.
type HTTPTransport struct {
	ctx   context.Context
	dmsgC *dmsg.Client
}

// MakeHTTPTransport makes an HTTPTransport.
func MakeHTTPTransport(ctx context.Context, dmsgC *dmsg.Client) HTTPTransport {
	return HTTPTransport{
		ctx:   ctx,
		dmsgC: dmsgC,
	}
}

// RoundTrip implements golang's http package support for alternative HTTP transport protocols.
// In this case dmsg is used instead of TCP to initiate the communication with the server.
func (t HTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var hostAddr dmsg.Addr
	if err := hostAddr.Set(req.Host); err != nil {
		return nil, fmt.Errorf("invalid host address: %w", err)
	}
	if hostAddr.Port == 0 {
		hostAddr.Port = defaultHTTPPort
	}

	// TODO(evanlinjin): In the future, we should implement stream reuse to save bandwidth.
	// We do not close the stream here as it is the user's responsibility to close the stream after resp.Body is fully
	// read.
	stream, err := t.dmsgC.DialStream(req.Context(), hostAddr)
	if err != nil {
		return nil, err
	}
	if err := req.Write(stream); err != nil {
		return nil, err
	}
	bufR := bufio.NewReader(stream)
	resp, err := http.ReadResponse(bufR, req)
	if err != nil {
		return nil, err
	}

	defer func() {
		go test(t.ctx, resp, stream)
	}()

	return resp, nil
}

func test(ctx context.Context, resp *http.Response, stream *dmsg.Stream) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := resp.Body.Read(nil)
			log := stream.Logger()
			log.Errorf("err %v", err)
			if err == nil {
				// can still read from body so it's not closed

			} else if err != nil && err.Error() == "http: invalid Read on closed Body" {
				stream.Close()
				return
			}
		}
	}

}
