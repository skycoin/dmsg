package dmsg

import (
	"net"
	"sync"

	"github.com/skycoin/dmsg/cipher"
)

// Listener listens for remote-initiated transports.
type Listener interface {
	net.Listener

	// AcceptTransport is similar to (net.Listener).Accept,
	// except that it returns a TransportInterface instead of a net.Conn.
	AcceptTransport() (TransportInterface, error)

	// Type returns the Transport type.
	Type() string
}

type listener struct {
	pk     cipher.PubKey
	port   uint16
	accept chan TransportInterface
	done   chan struct{}
	once   sync.Once
}

func (l *listener) Accept() (net.Conn, error) {
	return l.AcceptTransport()
}

func (l *listener) Close() error {
	l.once.Do(func() {
		close(l.done)
		for {
			select {
			case <-l.accept:
			default:
				close(l.accept)
				return
			}
		}
	})

	return nil
}

func (l *listener) Addr() net.Addr {
	return Addr{
		pk:   l.pk,
		port: &l.port,
	}
}

func (l *listener) AcceptTransport() (TransportInterface, error) {
	select {
	case tp, ok := <-l.accept:
		if !ok {
			return nil, ErrClientClosed
		}
		return tp, nil
	case <-l.done:
		return nil, ErrClientClosed
	}
}

func (l *listener) Type() string {
	return Type
}
