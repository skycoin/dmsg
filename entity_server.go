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

// ServerEntity represents a dsmg server entity.
type ServerEntity struct {
	EntityCommon
}

// NewServer creates a new dmsg server entity.
func NewServer(pk cipher.PubKey, sk cipher.SecKey, dc disc.APIClient) *ServerEntity {
	s := new(ServerEntity)
	s.EntityCommon.init(pk, sk, dc, logging.MustGetLogger("dmsg_server"))
	return s
}

// Serve serves the server.
func (s *ServerEntity) Serve(ctx context.Context, lis net.Listener, addr string) error {
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

func (s *ServerEntity) updateEntryLoop(ctx context.Context, timeout time.Duration, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	err := netutil.NewRetrier(s.log, 100*time.Millisecond, 0, 2).
		Do(ctx, func() error { return s.updateServerEntry(ctx, addr) })
	cancel()
	return err
}

func (s *ServerEntity) handleConn(ctx context.Context, conn net.Conn) {
	var log logrus.FieldLogger
	log = s.log.WithField("remote_tcp", conn.RemoteAddr())

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
		log.Info("Stopped serving session.")
	}()

	dSes, err := makeServerSession(&s.EntityCommon, conn)
	if err != nil {
		log = log.WithError(err)
		return
	}

	s.setSession(ctx, dSes.SessionCommon)
	defer func() {
		s.delSession(ctx, dSes.RemotePK())
		_ = dSes.Close() //nolint:errcheck
	}()

	log = log.WithField("remote_pk", dSes.RemotePK())
	log.Info("Serving session.")
	dSes.Serve()
}
