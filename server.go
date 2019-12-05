package dmsg

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/noise"
)

// ErrListenerAlreadyWrappedToNoise occurs when the provided net.Listener is already wrapped with noise.Listener
var ErrListenerAlreadyWrappedToNoise = errors.New("listener is already wrapped to *noise.Listener")

// Server represents a dms_server.
type Server struct {
	log *logging.Logger

	pk cipher.PubKey
	sk cipher.SecKey
	dc disc.APIClient

	addr string
	lis  net.Listener
	ss   map[cipher.PubKey]*Session
	mx   sync.RWMutex

	wg sync.WaitGroup

	lisDone  int32
	doneOnce sync.Once
}

// NewServer creates a new dmsg_server.
func NewServer(pk cipher.PubKey, sk cipher.SecKey, addr string, l net.Listener, dc disc.APIClient) (*Server, error) {
	if addr == "" {
		addr = l.Addr().String()
	}

	if _, ok := l.(*noise.Listener); ok {
		return nil, ErrListenerAlreadyWrappedToNoise
	}

	return &Server{
		log:  logging.MustGetLogger("dmsg_server"),
		pk:   pk,
		sk:   sk,
		addr: addr,
		lis:  noise.WrapListener(l, pk, sk, false, noise.HandshakeXK),
		dc:   dc,
		ss:   make(map[cipher.PubKey]*Session),
	}, nil
}

// SetLogger set's the logger.
func (s *Server) SetLogger(log *logging.Logger) {
	s.log = log
}

// Addr returns the server's listening address.
func (s *Server) Addr() string {
	return s.addr
}

// SessionGetter is a function that obtains a session.
type SessionGetter func(pk cipher.PubKey) (*Session, bool)

// Session obtains a session between the remote client and this server.
func (s *Server) Session(pk cipher.PubKey) (*Session, bool) {
	s.mx.RLock()
	dSes, ok := s.ss[pk]
	s.mx.RUnlock()
	return dSes, ok
}

func (s *Server) setSession(dSes *Session) {
	s.mx.Lock()
	s.ss[dSes.rPK] = dSes
	s.mx.Unlock()
}

func (s *Server) delSession(pk cipher.PubKey) {
	s.mx.Lock()
	delete(s.ss, pk)
	s.mx.Unlock()
}

func (s *Server) sessionCount() int {
	s.mx.RLock()
	n := len(s.ss)
	s.mx.RUnlock()
	return n
}

func (s *Server) close() (closed bool, err error) {
	s.doneOnce.Do(func() {
		closed = true
		atomic.StoreInt32(&s.lisDone, 1)

		if err = s.lis.Close(); err != nil {
			return
		}

		s.mx.Lock()
		s.ss = make(map[cipher.PubKey]*Session)
		s.mx.Unlock()
	})

	return closed, err
}

// Close closes the dmsg_server.
func (s *Server) Close() error {
	closed, err := s.close()
	if !closed {
		return errors.New("server is already closed")
	}
	if err != nil {
		return err
	}

	s.wg.Wait()
	return nil
}

func (s *Server) isLisClosed() bool {
	return atomic.LoadInt32(&s.lisDone) == 1
}

// Serve serves the dmsg_server.
func (s *Server) Serve() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.retryUpdateEntry(ctx, StreamHandshakeTimeout); err != nil {
		return fmt.Errorf("updating server's client entry failed with: %s", err)
	}

	s.log.
		WithField("local_addr", s.addr).
		WithField("local_pk", s.pk).
		Info("serving")

	for {
		conn, err := s.lis.Accept()
		if err != nil {
			// if the listener is closed, it means that this error is not interesting
			// for the outer client
			if s.isLisClosed() {
				return nil
			}
			// Continue if error is temporary.
			if err, ok := err.(net.Error); ok {
				if err.Temporary() {
					continue
				}
			}
			return err
		}

		s.wg.Add(1)
		go func(conn net.Conn) {
			defer func() {
				_ = conn.Close() //nolint:errcheck
				s.wg.Done()
			}()

			dSes, err := NewServerSession(s.log, s.Session, conn, s.sk, s.pk)
			if err != nil {
				s.log.WithError(err).Warn("failed to accept new session")
				return
			}
			s.setSession(dSes)

			s.log.
				WithField("remote_addr", conn.RemoteAddr()).
				WithField("remote_pk", dSes.RemotePK()).
				Info("serving session")

			for {
				if err := dSes.AcceptServerStream(); err != nil {
					s.log.Infof("connection with client %s closed: error(%v)", dSes.rPK, err)
					s.delSession(dSes.rPK)
					return
				}
			}
		}(conn)
	}
}

func (s *Server) updateDiscEntry(ctx context.Context) error {
	entry, err := s.dc.Entry(ctx, s.pk)
	if err != nil {
		entry = disc.NewServerEntry(s.pk, 0, s.addr, 10)
		if err := entry.Sign(s.sk); err != nil {
			fmt.Println("err in sign")
			return err
		}
		return s.dc.SetEntry(ctx, entry)
	}

	entry.Server.Address = s.Addr()
	s.log.Infoln("updatingEntry:", entry)

	return s.dc.UpdateEntry(ctx, s.sk, entry)
}

func (s *Server) retryUpdateEntry(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		if err := s.updateDiscEntry(ctx); err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				retry := time.Second
				s.log.WithError(err).Warnf("updateEntry failed: trying again in %d second...", retry)
				time.Sleep(retry)
				continue
			}
		}
		return nil
	}
}
