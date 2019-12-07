package dmsg

import (
	"context"
	"net"
	"sync"

	"github.com/SkycoinProject/skycoin/src/util/logging"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/netutil"
)

type Config struct {
	MinSessions int
}

func DefaultConfig() *Config {
	return &Config{
		MinSessions: 1,
	}
}

type ClientEntity struct {
	EntityCommon
	conf   *Config
	porter *netutil.Porter
	errCh  chan error
	done   chan struct{}
	once   sync.Once
}

func NewClient(pk cipher.PubKey, sk cipher.SecKey, dc disc.APIClient, conf *Config) *ClientEntity {
	if conf == nil {
		conf = DefaultConfig()
	}

	c := new(ClientEntity)
	c.conf = conf
	c.porter = netutil.NewPorter(netutil.PorterMinEphemeral)
	c.errCh = make(chan error)
	c.done = make(chan struct{})

	c.EntityCommon.init(pk, sk, dc, logging.MustGetLogger("dmsg_client"))
	c.EntityCommon.setSessionCallback = c.EntityCommon.updateClientEntry
	c.EntityCommon.delSessionCallback = c.EntityCommon.updateClientEntry

	return c
}

func (ce *ClientEntity) Serve() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ce.done
		cancel()
	}()

	for {
		ce.log.Info("Discovering dmsg servers...")
		entries, err := ce.discoverServers(ctx)
		if err != nil {
			ce.log.WithError(err).Warn("Failed to discover dmsg servers.")
			if isDone(ctx) {
				return
			}
		}

		for _, entry := range entries {
			// If session with server of pk already exists, skip.
			if _, ok := ce.ClientSession(ce.porter, entry.Static); ok {
				continue
			}

			sesCh := make(chan ClientSession, 1)
			go func() { ce.errCh <- ce.serveSession(ctx, entry, sesCh) }()
			select {
			case <-ce.done:
				return
			case dSes := <-sesCh:
				ce.log.WithField("remote_pk", dSes.RemotePK()).Info("Serving session.")
			}

			if ce.SessionCount() >= ce.conf.MinSessions {
				select {
				case <-ce.done:
					return
				case err := <-ce.errCh:
					ce.log.WithError(err).Info("Session stopped.")
				}
			}
		}
	}
}

func (ce *ClientEntity) discoverServers(ctx context.Context) (entries []*disc.Entry, err error) {
	err = netutil.NewDefaultRetrier(ce.log).Do(ctx, func() error {
		entries, err = ce.dc.AvailableServers(ctx)
		return err
	})
	return entries, err
}

// TODO(evanlinjin): Have waitgroup.
func (ce *ClientEntity) Close() error {
	if ce == nil {
		return nil
	}

	ce.once.Do(func() {
		ce.mx.Lock()
		defer ce.mx.Unlock()

		close(ce.done)

		for _, dSes := range ce.ss {
			ce.log.
				WithError(dSes.Close()).
				Info("Session closed.")
		}
		ce.ss = make(map[cipher.PubKey]*SessionCommon)

		ce.porter.RangePortValues(func(port uint16, v interface{}) (next bool) {
			switch v.(type) {
			case *Listener:
				ce.log.
					WithError(v.(*Listener).Close()).
					Info("Listener closed.")
			case *Stream2:
				ce.log.
					WithError(v.(*Stream2).Close()).
					Info("Stream closed.")
			}
			return true
		})
	})

	return nil
}

func (ce *ClientEntity) Listen(port uint16) (*Listener, error) {
	lis := newListener(Addr{PK: ce.pk, Port: port})
	ok, doneFn := ce.porter.Reserve(port, lis)
	if !ok {
		lis.close()
		return nil, ErrPortOccupied
	}
	lis.addCloseCallback(doneFn)
	return lis, nil
}

func (ce *ClientEntity) DialStream(ctx context.Context, addr Addr) (*Stream2, error) {
	entry, err := getClientEntry(ctx, ce.dc, addr.PK)
	if err != nil {
		return nil, err
	}

	// Range client's delegated servers.
	// See if we are already connected to a delegated server.
	for _, srvPK := range entry.Client.DelegatedServers {
		if dSes, ok := ce.ClientSession(ce.porter, srvPK); ok {
			return dSes.DialStream(addr)
		}
	}

	// Range client's delegated servers.
	// Attempt to connect to a delegated server.
	for _, srvPK := range entry.Client.DelegatedServers {
		srvEntry, err := getServerEntry(ctx, ce.dc, srvPK)
		if err != nil {
			continue
		}
		sesCh := make(chan ClientSession, 1)
		go func() { ce.errCh <- ce.serveSession(ctx, srvEntry, sesCh) }()
		select {
		case <-ce.done:
			return nil, ErrEntityClosed
		case <-ctx.Done():
			return nil, ctx.Err()
		case dSes := <-sesCh:
			return dSes.DialStream(addr)
		}
	}

	return nil, ErrCannotConnectToDelegated
}

// It is expected that the session is created and served before the context cancels, otherwise an error will be returned.
func (ce *ClientEntity) serveSession(ctx context.Context, entry *disc.Entry, sesCh chan<- ClientSession) error {
	conn, err := net.Dial("tcp", entry.Server.Address)
	if err != nil {
		return err
	}
	dSes, err := makeClientSession(&ce.EntityCommon, ce.porter, conn, entry.Static)
	if err != nil {
		return err
	}

	ce.setSession(ctx, dSes.SessionCommon)
	defer func() {
		ce.delSession(ctx, dSes.RemotePK())
		_ = dSes.Close() //nolint:errcheck
	}()

	notifyOfClientSession(ctx, sesCh, dSes)
	return dSes.Serve()
}
