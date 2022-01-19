package dmsghttp

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/skycoin/dmsg"
)

const defaultHTTPPort = uint16(80)

// HTTPTransport implements http.RoundTripper
// Do not confuse this with a Skywire Transport implementation.
type HTTPTransport struct {
	dmsgC     *dmsg.Client
	streamMap chan map[*http.Request]uint32
}

// MakeHTTPTransport makes an HTTPTransport.
func MakeHTTPTransport(dmsgC *dmsg.Client, streamMap chan map[*http.Request]uint32) HTTPTransport {

	return HTTPTransport{dmsgC: dmsgC, streamMap: streamMap}
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

	streamMap := make(map[*http.Request]uint32)
	streamMap[req] = stream.StreamID()
	t.streamMap <- streamMap

	if err := req.Write(stream); err != nil {
		return nil, err
	}
	bufR := bufio.NewReader(stream)
	return http.ReadResponse(bufR, req)
}
