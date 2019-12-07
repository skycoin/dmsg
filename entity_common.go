package dmsg

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/netutil"
)

type EntityCommon struct {
	pk cipher.PubKey
	sk cipher.SecKey
	dc disc.APIClient

	ss map[cipher.PubKey]*Session
	mx *sync.Mutex

	log logrus.FieldLogger

	setSessionCallback func(ctx context.Context) error
	delSessionCallback func(ctx context.Context) error
}

func (c *EntityCommon) init(pk cipher.PubKey, sk cipher.SecKey, dc disc.APIClient, log logrus.FieldLogger) {
	c.pk = pk
	c.sk = sk
	c.dc = dc
	c.ss = make(map[cipher.PubKey]*Session)
	c.mx = new(sync.Mutex)
	c.log = log
}

func (c *EntityCommon) LocalPK() cipher.PubKey { return c.pk }

func (c *EntityCommon) SetLogger(log logrus.FieldLogger) { c.log = log }

func (c *EntityCommon) Session(pk cipher.PubKey) (*Session, bool) {
	c.mx.Lock()
	dSes, ok := c.ss[pk]
	c.mx.Unlock()
	return dSes, ok
}

func (c *EntityCommon) SessionCount() int {
	c.mx.Lock()
	n := len(c.ss)
	c.mx.Unlock()
	return n
}

func (c *EntityCommon) setSession(ctx context.Context, dSes *Session) {
	c.mx.Lock()
	c.ss[dSes.RemotePK()] = dSes
	if c.setSessionCallback != nil {
		if err := c.setSessionCallback(ctx); err != nil {
			c.log.
				WithError(err).
				Warn("setSession() callback returned non-nil error.")
		}
	}
	c.mx.Unlock()
}

func (c *EntityCommon) delSession(ctx context.Context, pk cipher.PubKey) {
	c.mx.Lock()
	delete(c.ss, pk)
	if c.delSessionCallback != nil {
		if err := c.delSessionCallback(ctx); err != nil {
			c.log.
				WithError(err).
				Warn("delSession() callback returned non-nil error.")
		}
	}
	c.mx.Unlock()
}

// updateServerEntry updates the dmsg server's entry within dmsg discovery.
func (c *EntityCommon) updateServerEntry(ctx context.Context, addr string) error {
	entry, err := c.dc.Entry(ctx, c.pk)
	if err != nil {
		entry = disc.NewServerEntry(c.pk, 0, addr, 10)
		if err := entry.Sign(c.sk); err != nil {
			fmt.Println("err in sign")
			return err
		}
		return c.dc.SetEntry(ctx, entry)
	}
	entry.Server.Address = addr
	return c.dc.UpdateEntry(ctx, c.sk, entry)
}

func (c *EntityCommon) updateClientEntry(ctx context.Context) error {
	srvPKs := make([]cipher.PubKey, 0, len(c.ss))
	for pk := range c.ss {
		srvPKs = append(srvPKs, pk)
	}
	entry, err := c.dc.Entry(ctx, c.pk)
	if err != nil {
		entry = disc.NewClientEntry(c.pk, 0, srvPKs)
		if err := entry.Sign(c.sk); err != nil {
			return err
		}
		return c.dc.SetEntry(ctx, entry)
	}
	entry.Client.DelegatedServers = srvPKs
	c.log.Infoln("updatingEntry:", entry)
	return c.dc.UpdateEntry(ctx, c.sk, entry)
}

// It is expected that the session is created and served before the context cancels, otherwise an error will be returned.
func (c *EntityCommon) initiateAndServeSession(ctx context.Context, porter *netutil.Porter, entry *disc.Entry, sesCh chan<- *Session) error {

	conn, err := net.Dial("tcp", entry.Server.Address)
	if err != nil {
		return err
	}
	dSes, err := InitiateSession(c.log, porter, conn, c.sk, c.pk, entry.Static)
	if err != nil {
		return err
	}

	c.setSession(ctx, dSes)
	defer func() {
		c.delSession(ctx, dSes.RemotePK())
		_ = dSes.Close() //nolint:errcheck
	}()

	notifyOfSession(ctx, sesCh, dSes)
	for {
		if err := dSes.acceptClientStream(); err != nil {
			return err
		}
	}
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func notifyOfSession(ctx context.Context, sesCh chan<- *Session, dSes *Session) {
	if sesCh != nil {
		select {
		case sesCh <- dSes:
		case <-ctx.Done():
		}
		close(sesCh)
	}
}

func getServerEntry(ctx context.Context, dc disc.APIClient, srvPK cipher.PubKey) (*disc.Entry, error) {
	entry, err := dc.Entry(ctx, srvPK)
	if err != nil {
		return nil, ErrDiscEntryNotFound
	}
	if entry.Server == nil {
		return nil, ErrDiscEntryIsNotServer
	}
	return entry, nil
}

func getClientEntry(ctx context.Context, dc disc.APIClient, clientPK cipher.PubKey) (*disc.Entry, error) {
	entry, err := dc.Entry(ctx, clientPK)
	if err != nil {
		return nil, ErrDiscEntryNotFound
	}
	if entry.Client == nil {
		return nil, ErrDiscEntryIsNotClient
	}
	if len(entry.Client.DelegatedServers) == 0 {
		return nil, ErrDiscEntryHasNoDelegated
	}
	return entry, nil
}

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

func NewClientEntity(pk cipher.PubKey, sk cipher.SecKey, dc disc.APIClient, conf *Config) *ClientEntity {
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
			if _, ok := ce.Session(entry.Static); ok {
				continue
			}

			sesCh := make(chan *Session, 1)
			go func() { ce.errCh <- ce.initiateAndServeSession(ctx, ce.porter, entry, sesCh) }()
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
		ce.ss = make(map[cipher.PubKey]*Session)

		ce.porter.RangePortValues(func(port uint16, v interface{}) (next bool) {
			switch v.(type) {
			case *Listener:
				ce.log.
					WithError(v.(*Listener).Close()).
					Info("Listener closed.")
			case *Stream:
				ce.log.
					WithError(v.(*Stream).Close()).
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
	lis.doneFunc = doneFn
	return lis, nil
}

func (ce *ClientEntity) DialStream(ctx context.Context, addr Addr) (*Stream, error) {
	entry, err := getClientEntry(ctx, ce.dc, addr.PK)
	if err != nil {
		return nil, err
	}

	// Range client's delegated servers.
	// See if we are already connected to a delegated server.
	for _, srvPK := range entry.Client.DelegatedServers {
		if dSes, ok := ce.Session(srvPK); ok {
			return dSes.dialClientStream(ctx, addr)
		}
	}

	// Range client's delegated servers.
	// Attempt to connect to a delegated server.
	for _, srvPK := range entry.Client.DelegatedServers {
		srvEntry, err := getServerEntry(ctx, ce.dc, srvPK)
		if err != nil {
			continue
		}
		sesCh := make(chan *Session, 1)
		go func() { ce.errCh <- ce.initiateAndServeSession(ctx, ce.porter, srvEntry, sesCh) }()
		select {
		case <-ce.done:
			return nil, ErrEntityClosed
		case <-ctx.Done():
			return nil, ctx.Err()
		case dSes := <-sesCh:
			return dSes.dialClientStream(ctx, addr)
		}
	}

	return nil, ErrCannotConnectToDelegated
}
