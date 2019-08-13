package dmsg

import (
	"net"
	"sync"

	"github.com/skycoin/dmsg/cipher"
)

// Listener listens for remote-initiated transports.
type Listener struct {
	pk       cipher.PubKey
	port     uint16
	acceptMu sync.Mutex // protects 'accept'
	accept   chan *Transport
	done     chan struct{}
	once     sync.Once
}

// Accept accepts a connection.
func (l *Listener) Accept() (net.Conn, error) {
	return l.AcceptTransport()
}

// Close closes the listener.
func (l *Listener) Close() error {
	l.once.Do(func() {
		close(l.done)
		l.close()
	})

	return nil
}

func (l *Listener) close() {
	l.acceptMu.Lock()
	defer l.acceptMu.Unlock()

	for {
		select {
		case <-l.accept:
		default:
			close(l.accept)
			return
		}
	}
}

// Addr returns the listener's address.
func (l *Listener) Addr() net.Addr {
	return Addr{
		pk:   l.pk,
		port: &l.port,
	}
}

// AcceptTransport accepts a transport connection.
func (l *Listener) AcceptTransport() (*Transport, error) {
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

// Type returns the transport type.
func (l *Listener) Type() string {
	return Type
}

func (l *Listener) IntroduceTransport(tp *Transport) error {
	l.acceptMu.Lock()
	defer l.acceptMu.Unlock()

	select {
	case l.accept <- tp:
		if err := tp.WriteAccept(); err != nil {
			return err
		}
		go tp.Serve()
		return nil

	default:
		if err := tp.Close(); err != nil {
			log.WithError(err).Warn("Failed to close transport")
		}
		return ErrClientAcceptMaxed
	}
}
