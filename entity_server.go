package dmsg

import (
	"context"
	"net"
	"sync"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/netutil"
)

// ServerEntity represents a dsmg server entity.
type ServerEntity struct {
	EntityCommon
	done chan struct{}
	once sync.Once
	wg   sync.WaitGroup
}

// NewServer creates a new dmsg server entity.
func NewServer(pk cipher.PubKey, sk cipher.SecKey, dc disc.APIClient) *ServerEntity {
	s := new(ServerEntity)
	s.EntityCommon.init(pk, sk, dc, logging.MustGetLogger("dmsg_server"))
	s.done = make(chan struct{})
	return s
}

func (s *ServerEntity) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		close(s.done)
		s.wg.Wait()
	})
	return nil
}

// Serve serves the server.
func (s *ServerEntity) Serve(lis net.Listener, addr string) error {
	var log logrus.FieldLogger
	log = s.log.WithField("local_addr", addr).WithField("local_pk", s.pk)

	log.Info("Serving server.")
	s.wg.Add(1)

	defer func() {
		log.Info("Stopped server.")
		s.wg.Done()
	}()

	go func() {
		<-s.done
		log.Info("Stopping server...")
		_ = lis.Close() //nolint:errcheck
	}()

	log.Info("Updating discovery entry...")
	if addr == "" {
		addr = lis.Addr().String()
	}
	if err := s.updateEntryLoop(addr); err != nil {
		return err
	}

	log.Info("Accepting sessions...")
	for {
		conn, err := lis.Accept()
		if err != nil {
			// If server is closed, there is no error to report.
			if isClosed(s.done) {
				return nil
			}
			return err
		}

		s.wg.Add(1)
		go func() {
			s.handleSession(conn)
			s.wg.Done()
		}()
	}
}

func (s *ServerEntity) updateEntryLoop(addr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		awaitDone(ctx, s.done)
		cancel()
	}()
	return netutil.NewDefaultRetrier(s.log).Do(ctx, func() error { return s.updateServerEntry(ctx, addr) })
}

func (s *ServerEntity) handleSession(conn net.Conn) {
	var log logrus.FieldLogger
	log = s.log.WithField("remote_tcp", conn.RemoteAddr())

	dSes, err := makeServerSession(&s.EntityCommon, conn)
	if err != nil {
		log = log.WithError(err)
		_ = conn.Close() //nolint:errcheck
		return
	}

	log = log.WithField("remote_pk", dSes.RemotePK())
	log.Info("Started session.")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		awaitDone(ctx, s.done)
		_ = dSes.Close() //nolint:errcheck
		log.Info("Stopped session.")
	}()

	s.setSession(ctx, dSes.SessionCommon)
	dSes.Serve()
	s.delSession(ctx, dSes.RemotePK())

	cancel()
}

func awaitDone(ctx context.Context, done chan struct{}) {
	select {
	case <-ctx.Done():
	case <-done:
	}
	return
}
