package dmsg

import (
	"net"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/noise"
)

type Stream2 struct {
	lAddr Addr // local address
	rAddr Addr // remote address
	sk  cipher.SecKey     // Local secret key.
	ys  *yamux.Stream     // Underlying yamux stream.
	ns  *noise.ReadWriter // Underlying noise read writer.
	log logrus.FieldLogger
}

func (s *Stream2) LocalAddr() net.Addr {
	return s.lAddr
}

func (s *Stream2) RemoteAddr() net.Addr {
	return s.rAddr
}

func (s *Stream2) Read(b []byte) (int, error) {
	return s.ns.Read(b)
}

func (s *Stream2) Write(b []byte) (int, error) {
	return s.ns.Write(b)
}

func (s *Stream2) SetDeadline(t time.Time) error {
	return s.ys.SetDeadline(t)
}

func (s *Stream2) SetReadDeadline(t time.Time) error {
	return s.ys.SetReadDeadline(t)
}

func (s *Stream2) SetWriteDeadline(t time.Time) error {
	return s.ys.SetWriteDeadline(t)
}

func (s *Stream2) Close() error {
	return s.ys.Close()
}
