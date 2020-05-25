package dmsghttp

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/SkycoinProject/dmsg"
)

const defaultHTTPPort = uint16(80)

// HTTPTransport implements http.RoundTripper
// Do not confuse this with a Skywire Transport implementation.
type HTTPTransport struct {
	dmsgC *dmsg.Client
}

// MakeHTTPTransport makes an HTTPTransport.
func MakeHTTPTransport(dmsgC *dmsg.Client) HTTPTransport {
	return HTTPTransport{dmsgC: dmsgC}
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
	stream, err := t.dmsgC.DialStream(req.Context(), hostAddr)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := stream.Close()
		t.dmsgC.Logger().WithError(err).WithField("func", "HTTPTransport.RoundTrip").
			Debug("Underlying stream closed.")
	}()

	if err := req.Write(stream); err != nil {
		return nil, err
	}
	return http.ReadResponse(bufio.NewReader(stream), req)
}
