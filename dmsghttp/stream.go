package dmsghttp

import (
	"context"
	"net/http"
	"sync"

	"github.com/skycoin/dmsg"
)

// StreamCloser saves a map
type StreamCloser struct {
	streamMap map[*http.Request]uint32
	dmsgC     *dmsg.Client
	mu        sync.Mutex
}

// NewStreamCloser gives a new stream closer
func NewStreamCloser(dmsgC *dmsg.Client) *StreamCloser {
	sMap := make(map[*http.Request]uint32)

	sCloser := &StreamCloser{
		streamMap: sMap,
		dmsgC:     dmsgC,
	}
	return sCloser
}

// CloseStream closes the stream associated with the http req
func (sc *StreamCloser) CloseStream(req *http.Request) error {
	sc.mu.Lock()
	streamID := sc.streamMap[req]
	sc.mu.Unlock()
	streams := sc.dmsgC.AllStreams()
	for _, stream := range streams {
		if streamID == stream.StreamID() {
			if err := stream.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetMap gets the http request and the stream ID associated it with and save it
func (sc *StreamCloser) GetMap(ctx context.Context, sMap chan map[*http.Request]uint32) {
	for {
		select {
		case <-ctx.Done():
			return
		case streamMap := <-sMap:
			sc.mu.Lock()
			sc.streamMap = streamMap
			sc.mu.Unlock()
		}
	}
}
