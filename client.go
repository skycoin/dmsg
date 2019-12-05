package dmsg

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/netutil"
)

const (
	clientReconnectInterval = 3 * time.Second
)

var (
	// ErrNoSrv indicate that remote client does not have DelegatedServers in entry.
	ErrNoSrv = errors.New("remote has no DelegatedServers")
	// ErrClientClosed indicates that client is closed and not accepting new connections.
	ErrClientClosed = errors.New("client closed")
	// ErrClientAcceptMaxed indicates that the client cannot take in more accepts.
	ErrClientAcceptMaxed = errors.New("client accepts buffer maxed")
)

// ClientOption represents an optional argument for Client.
type ClientOption func(c *Client) error

// SetLogger sets the internal logger for Client.
func SetLogger(log *logging.Logger) ClientOption {
	return func(c *Client) error {
		if log == nil {
			return errors.New("nil logger set")
		}
		c.log = log
		return nil
	}
}

// Client implements stream.Factory
type Client struct {
	log *logging.Logger

	pk cipher.PubKey
	sk cipher.SecKey
	dc disc.APIClient

	ss map[cipher.PubKey]*Session // sessions with messaging servers. Key: pk of server
	mx sync.RWMutex

	pm *netutil.Porter

	done chan struct{}
	once sync.Once
}

// NewClient creates a new Client.
func NewClient(pk cipher.PubKey, sk cipher.SecKey, dc disc.APIClient, opts ...ClientOption) *Client {
	c := &Client{
		log:  logging.MustGetLogger("dmsg_client"),
		pk:   pk,
		sk:   sk,
		dc:   dc,
		ss:   make(map[cipher.PubKey]*Session),
		pm:   netutil.NewPorter(netutil.PorterMinEphemeral),
		done: make(chan struct{}),
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			panic(err)
		}
	}
	return c
}

func (c *Client) updateDiscEntry(ctx context.Context) error {
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

func (c *Client) setSession(ctx context.Context, conn *Session) {
	c.mx.Lock()
	c.ss[conn.rPK] = conn
	if err := c.updateDiscEntry(ctx); err != nil {
		c.log.WithError(err).Warn("updateEntry: failed")
	}
	c.mx.Unlock()
}

func (c *Client) delSession(ctx context.Context, pk cipher.PubKey) {
	c.mx.Lock()
	delete(c.ss, pk)
	if err := c.updateDiscEntry(ctx); err != nil {
		c.log.WithError(err).Warn("updateEntry: failed")
	}
	c.mx.Unlock()
}

func (c *Client) getSession(pk cipher.PubKey) (*Session, bool) {
	c.mx.RLock()
	l, ok := c.ss[pk]
	c.mx.RUnlock()
	return l, ok
}

func (c *Client) sessionCount() int {
	c.mx.RLock()
	n := len(c.ss)
	c.mx.RUnlock()
	return n
}

// InitiateServerConnections initiates connections with dms_servers.
func (c *Client) InitiateServerConnections(ctx context.Context, min int) error {
	if min == 0 {
		return nil
	}
	entries, err := c.findServerEntries(ctx)
	if err != nil {
		return err
	}
	c.log.Info("found dmsg_server entries:", entries)
	if err := c.findOrConnectToServers(ctx, entries, min); err != nil {
		return err
	}
	return nil
}

func (c *Client) findServerEntries(ctx context.Context) ([]*disc.Entry, error) {
	for {
		entries, err := c.dc.AvailableServers(ctx)
		if err != nil || len(entries) == 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("dmsg_servers are not available: %s", err)
			default:
				retry := time.Second
				c.log.WithError(err).Warnf("no dmsg_servers found: trying again in %v...", retry)
				time.Sleep(retry)
				continue
			}
		}
		return entries, nil
	}
}

func (c *Client) findOrConnectToServers(ctx context.Context, entries []*disc.Entry, min int) error {
	for _, entry := range entries {
		_, err := c.findOrConnectToServer(ctx, entry.Static)
		if err != nil {
			c.log.Warnf("findOrConnectToServers: failed to find/connect to server %s: %s", entry.Static, err)
			continue
		}
		c.log.Infof("findOrConnectToServers: found/connected to server %s", entry.Static)
		if c.sessionCount() >= min {
			return nil
		}
	}
	return fmt.Errorf("findOrConnectToServers: all servers failed")
}

func (c *Client) findOrConnectToServer(ctx context.Context, srvPK cipher.PubKey) (*Session, error) {
	if dSes, ok := c.getSession(srvPK); ok {
		return dSes, nil
	}

	entry, err := c.dc.Entry(ctx, srvPK)
	if err != nil {
		return nil, err
	}
	if entry.Server == nil {
		return nil, errors.New("entry is of client instead of server")
	}

	conn, err := net.Dial("tcp", entry.Server.Address)
	if err != nil {
		return nil, err
	}
	c.log.WithField("server_pk", srvPK).Info("Connecting to server.")
	dSes, err := NewClientSession(c.log, c.pm, conn, c.sk, c.pk, srvPK)
	if err != nil {
		return nil, err
	}
	c.setSession(ctx, dSes)
	c.log.
		WithField("remote_pk", dSes.RemotePK()).
		Info("session created successfully")

	go func(dSes *Session) {
		// serve
		for {
			if err := dSes.AcceptClientStream(ctx); err != nil {
				dSes.log.
					WithError(err).
					WithField("remote_pk", dSes.RemotePK()).Debug("session with server closed")
				c.delSession(ctx, srvPK)
				break
			}
		}

		// reconnect logic
	retryServerConnect:
		select {
		case <-c.done:
		case <-ctx.Done():
		case <-time.After(clientReconnectInterval):
			dSes.log.WithField("remoteServer", srvPK).Warn("Reconnecting")
			if _, err := c.findOrConnectToServer(ctx, srvPK); err != nil {
				dSes.log.WithError(err).WithField("remoteServer", srvPK).Warn("ReconnectionFailed")
				goto retryServerConnect
			}
			dSes.log.WithField("remoteServer", srvPK).Warn("ReconnectionSucceeded")
		}
	}(dSes)

	return dSes, nil
}

// Listen creates a listener on a given port, adds it to port manager and returns the listener.
func (c *Client) Listen(port uint16) (*Listener, error) {
	lis := newListener(Addr{PK: c.pk, Port: port})
	ok, doneFunc := c.pm.Reserve(port, lis)
	if !ok {
		lis.close()
		return nil, errors.New("port occupied") // TODO(evanlinjin): Have proper error here.
	}
	lis.doneFunc = doneFunc
	return lis, nil
}

// DialClientStream dials a stream to remote dms_client.
func (c *Client) DialStream(ctx context.Context, remote cipher.PubKey, port uint16) (*Stream, error) {
	entry, err := c.dc.Entry(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("get entry failure: %s", err)
	}
	if entry.Client == nil {
		return nil, errors.New("entry is of server instead of client")
	}
	if len(entry.Client.DelegatedServers) == 0 {
		return nil, ErrNoSrv
	}
	for _, srvPK := range entry.Client.DelegatedServers {
		dSes, err := c.findOrConnectToServer(ctx, srvPK)
		if err != nil {
			c.log.WithError(err).Warn("failed to connect to server")
			continue
		}
		return dSes.DialClientStream(ctx, Addr{PK: remote, Port: port})
	}
	return nil, errors.New("failed to find dmsg_servers for given client pk")
}

// Addr returns the local dmsg_client's public key.
func (c *Client) Addr() net.Addr {
	return Addr{
		PK: c.pk,
	}
}

// Type returns the stream type.
func (c *Client) Type() string {
	return Type
}

// Close closes the dmsg_client and associated connections.
// TODO(evaninjin): proper error handling.
func (c *Client) Close() (err error) {
	if c == nil {
		return nil
	}

	c.once.Do(func() {
		close(c.done)

		c.mx.Lock()
		for _, dSes := range c.ss {
			if err := dSes.Close(); err != nil {
				c.log.WithField("reason", err).Debug("Session closed")
			}
		}
		c.ss = make(map[cipher.PubKey]*Session)
		c.mx.Unlock()
	})

	return err
}
