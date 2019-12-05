package dmsg

import (
	"context"
	"fmt"
	"net"
	"sync"
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
	pk cipher.PubKey
	sk cipher.SecKey
	dc disc.APIClient

	ss map[cipher.PubKey]*Session
	mx sync.Mutex

	log logrus.FieldLogger
}

func NewServer(pk cipher.PubKey, sk cipher.SecKey, dc disc.APIClient) *Server {
	return &Server{
		pk:  pk,
		sk:  sk,
		dc:  dc,
		ss:  make(map[cipher.PubKey]*Session),
		log: logging.MustGetLogger("dmsg_server"),
	}
}

// SetLogger should not be called after dmsg server is serving.
func (s *Server) SetLogger(log logrus.FieldLogger) { s.log = log }

func (s *Server) Session(pk cipher.PubKey) (*Session, bool) {
	s.mx.Lock()
	dSes, ok := s.ss[pk]
	s.mx.Unlock()
	return dSes, ok
}

func (s *Server) setSession(dSes *Session) {
	s.mx.Lock()
	s.ss[dSes.rPK] = dSes
	s.mx.Unlock()
}

func (s *Server) deleteSession(pk cipher.PubKey) {
	s.mx.Lock()
	delete(s.ss, pk)
	s.mx.Unlock()
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

	for {
		conn, err := lis.Accept()
		if err != nil {
			// If context is cancelled, there is no error to report.
			if isDone(ctx) {
				return nil
			}
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	var log logrus.FieldLogger
	log = s.log.WithField("remote_tcp", conn.RemoteAddr())

	defer func() {
		_ = conn.Close()
		log.Info("Stopped serving session.")
	}()

	dSes, err := NewServerSession(s.log, s.Session, conn, s.sk, s.pk)
	if err != nil {
		log = log.WithError(err)
		return
	}

	s.setSession(dSes)
	defer func() {
		s.deleteSession(dSes.RemotePK())
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

func (s *Server) updateEntryLoop(ctx context.Context, timeout time.Duration, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return netutil.NewRetrier(s.log, 100*time.Millisecond, 0, 2).
		Do(ctx, func() error {
			return UpdateServerEntry(ctx, s.dc, s.pk, s.sk, addr)
		})
}

// UpdateServerEntry updates the dmsg server's entry within dmsg discovery.
func UpdateServerEntry(ctx context.Context, dc disc.APIClient, pk cipher.PubKey, sk cipher.SecKey, addr string) error {
	entry, err := dc.Entry(ctx, pk)
	if err != nil {
		entry = disc.NewServerEntry(pk, 0, addr, 10)
		if err := entry.Sign(sk); err != nil {
			fmt.Println("err in sign")
			return err
		}
		return dc.SetEntry(ctx, entry)
	}
	entry.Server.Address = addr
	return dc.UpdateEntry(ctx, sk, entry)
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
