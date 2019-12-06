package dmsg

import (
	"context"
	"net"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/netutil"
)

// SessionGetter is a function that obtains a session.
type SessionGetter func(pk cipher.PubKey) (*Session, bool)

type Server struct {
	EntityCommon
}

func NewServer(pk cipher.PubKey, sk cipher.SecKey, dc disc.APIClient) *Server {
	s := new(Server)
	s.EntityCommon.init(pk, sk, dc, logging.MustGetLogger("dmsg_server"))
	return s
}

// Serve serves the server.
func (s *Server) Serve(ctx context.Context, lis net.Listener, addr string) error {
	if addr == "" {
		addr = lis.Addr().String()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := s.updateEntryLoop(ctx, time.Second*10, addr); err != nil {
		return err
	}

	s.log.
		WithField("local_addr", addr).
		WithField("local_pk", s.pk).
		Info("Serving dmsg server.")

	defer func() {
		s.log.
			WithField("local_addr", addr).
			WithField("local_pk", s.pk).
			Info("Stopped dmsg server.")
	}()

	for {
		conn, err := lis.Accept()
		if err != nil {
			// If context is cancelled, there is no error to report.
			if isDone(ctx) {
				return nil
			}
			return err
		}
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) updateEntryLoop(ctx context.Context, timeout time.Duration, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	err := netutil.NewRetrier(s.log, 100*time.Millisecond, 0, 2).
		Do(ctx, func() error { return s.updateServerEntry(ctx, addr) })
	cancel()
	return err
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	var log logrus.FieldLogger
	log = s.log.WithField("remote_tcp", conn.RemoteAddr())

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
		log.Info("Stopped serving session.")
	}()

	dSes, err := RespondSession(s.log, s.Session, conn, s.sk, s.pk)
	if err != nil {
		log = log.WithError(err)
		return
	}

	s.setSession(ctx, dSes)
	defer func() {
		s.delSession(ctx, dSes.RemotePK())
		_ = dSes.Close() //nolint:errcheck
	}()

	log = log.WithField("remote_pk", dSes.RemotePK())
	log.Info("Serving session.")

	for {
		if err := dSes.AcceptServerStream(); err != nil {
			log = log.WithError(err)
			return
		}
	}
}
