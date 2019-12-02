package dmsg

import (
	"bufio"
	"context"
	"net"

	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/netutil"
	"github.com/SkycoinProject/dmsg/noise"
)

// Session handles the multiplexed connection between the dmsg server and dmsg client.
type Session struct {
	lPK cipher.PubKey
	lSK cipher.SecKey
	rPK cipher.PubKey // Public key of the remote dmsg server.

	ys     *yamux.Session
	ns     *noise.Noise // For encrypting session messages, not stream messages.
	porter *netutil.Porter

	log logrus.FieldLogger
}

func NewClientSession(log logrus.FieldLogger, porter *netutil.Porter, conn net.Conn, lSK cipher.SecKey, lPK, rPK cipher.PubKey) (*Session, error) {
	ySes, err := yamux.Client(conn, yamux.DefaultConfig())
	if err != nil {
		return nil, err
	}
	ns, err := noise.New(noise.HandshakeXK, noise.Config{
		LocalPK:   lPK,
		LocalSK:   lSK,
		RemotePK:  rPK,
		Initiator: true,
	})
	if err != nil {
		return nil, err
	}
	if err := noise.InitiatorHandshake(ns, bufio.NewReader(conn), conn); err != nil {
		return nil, err
	}
	return &Session{
		lPK:    lPK,
		lSK:    lSK,
		rPK:    rPK,
		ys:     ySes,
		ns:     ns,
		porter: porter,
		log:    log,
	}, nil
}

func (s *Session) DialStream(ctx context.Context, dst Addr) (*Stream, error) {
	// Prepare yamux stream.
	ys, err := s.ys.OpenStream()
	if err != nil {
		return nil, err
	}
	// Prepare dmsg stream to reserve in porter.
	dstr := NewStream(ys, s.lSK, Addr{PK: s.lPK}, dst)
	if err := dstr.DoClientHandshake(ctx, s.log, s.porter, s.ns, dstr.ClientInitiatingHandshake); err != nil {
		return nil, err
	}
	return dstr, nil
}

func (s *Session) AcceptStream(ctx context.Context) error {
	ys, err := s.ys.AcceptStream()
	if err != nil {
		return err
	}
	dstr := NewStream(ys, s.lSK, Addr{PK: s.lPK}, Addr{})
	if err := dstr.DoClientHandshake(ctx, s.log, s.porter, s.ns, dstr.ClientRespondingHandshake); err != nil {
		return err
	}
	return nil
}

func (s *Session) Close() error {
	_ = s.ys.GoAway() //nolint:errcheck
	s.porter.RangePortValues(func(port uint16, v interface{}) (next bool) {
		switch v.(type) {
		case *Listener:
			_ = v.(*Listener).Close() //nolint:errcheck
		case *Stream:
			_ = v.(*Stream).Close() //nolint:errcheck
		}
		return true
	})
	return s.ys.Close()
}
