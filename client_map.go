package dmsg

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/disc"
	"github.com/skycoin/dmsg/noise"

	"github.com/skycoin/skycoin/src/util/logging"
)

// ClientConnMap represents a connection between a dmsg.Client and dmsg.Server from a client's perspective.
type ClientConnMap struct {
	log *logging.Logger

	net.Conn                // conn to dmsg server
	local     cipher.PubKey // local client's pk
	remoteSrv cipher.PubKey // dmsg server's public key

	// nextInitID keeps track of unused tp_ids to assign a future locally-initiated tp.
	// locally-initiated tps use an even tp_id between local and intermediary dms_server.
	nextInitID uint16

	// Transports: map of transports to remote dms_clients (key: tp_id, val: transport).
	tps map[uint16]*TransportMap
	mx  sync.RWMutex // to protect tps

	done chan struct{}
	once sync.Once
	wg   sync.WaitGroup
}

// NewClientConnMap creates a new ClientConnMap.
func NewClientConnMap(log *logging.Logger, conn net.Conn, local, remote cipher.PubKey) *ClientConnMap {
	cc := &ClientConnMap{
		log:        log,
		Conn:       conn,
		local:      local,
		remoteSrv:  remote,
		nextInitID: randID(true),
		done:       make(chan struct{}),
		tps:        make(map[uint16]*TransportMap),
	}
	cc.wg.Add(1)
	return cc
}

func (c *ClientConnMap) getNextInitID(ctx context.Context) (uint16, error) {
	for {
		select {
		case <-c.done:
			return 0, ErrClientClosed
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			if ch := c.tps[c.nextInitID]; ch != nil && !ch.IsClosed() {
				c.nextInitID += 2
				continue
			}
			c.tps[c.nextInitID] = nil
			id := c.nextInitID
			c.nextInitID = id + 2
			return id, nil
		}
	}
}

func (c *ClientConnMap) addTp(ctx context.Context, clientPK cipher.PubKey) (*TransportMap, error) {
	c.mx.Lock()
	defer c.mx.Unlock()

	id, err := c.getNextInitID(ctx)
	if err != nil {
		return nil, err
	}
	tp := NewTransportMap(c.Conn, c.log, c.local, clientPK, id, c.delTp)
	c.tps[id] = tp
	return tp, nil
}

func (c *ClientConnMap) setTp(tp *TransportMap) {
	c.mx.Lock()
	c.tps[tp.id] = tp
	c.mx.Unlock()
}

func (c *ClientConnMap) delTp(id uint16) {
	c.mx.Lock()
	c.tps[id] = nil
	c.mx.Unlock()
}

func (c *ClientConnMap) getTp(id uint16) (*TransportMap, bool) {
	c.mx.RLock()
	tp := c.tps[id]
	c.mx.RUnlock()
	ok := tp != nil && !tp.IsClosed()
	return tp, ok
}

func (c *ClientConnMap) close() (closed bool) {
	c.once.Do(func() {
		closed = true
		c.log.WithField("remoteServer", c.remoteSrv).Infoln("ClosingConnection")
		close(c.done)
		c.mx.Lock()
		for _, tp := range c.tps {
			if tp != nil {
				go tp.Close() //nolint:errcheck
			}
		}
		_ = c.Conn.Close() //nolint:errcheck
		c.mx.Unlock()
	})
	return closed
}

func (c *ClientConnMap) readOK() error {
	fr, err := readFrame(c.Conn)
	if err != nil {
		return errors.New("failed to get OK from server")
	}

	ft, _, _ := fr.Disassemble()
	if ft != OkType {
		return fmt.Errorf("wrong frame from server: %v", ft)
	}

	return nil
}

func (c *ClientConnMap) handleRequestFrame(accept chan<- *TransportMap, id uint16, p []byte) (cipher.PubKey, error) {
	// remotely-initiated tps should:
	// - have a payload structured as 'init_pk:resp_pk'.
	// - resp_pk should be of local client.
	// - use an odd tp_id with the intermediary dmsg_server.
	initPK, respPK, ok := splitPKs(p)
	if !ok || respPK != c.local || isInitiatorID(id) {
		if err := writeCloseFrame(c.Conn, id, 0); err != nil {
			return initPK, err
		}
		return initPK, ErrRequestCheckFailed
	}

	tp := NewTransportMap(c.Conn, c.log, c.local, initPK, id, c.delTp)

	select {
	case <-c.done:
		_ = tp.Close() //nolint:errcheck
		return initPK, ErrClientClosed

	case accept <- tp:
		c.setTp(tp)
		if err := tp.WriteAccept(); err != nil {
			return initPK, err
		}
		go tp.Serve()
		return initPK, nil

	default:
		_ = tp.Close() //nolint:errcheck
		return initPK, ErrClientAcceptMaxed
	}
}

// Serve handles incoming frames.
// Remote-initiated tps that are successfully created are pushing into 'accept' and exposed via 'Client.Accept()'.
func (c *ClientConnMap) Serve(ctx context.Context, accept chan<- *TransportMap) (err error) {
	log := c.log.WithField("remoteServer", c.remoteSrv)
	log.WithField("connCount", incrementServeCount()).Infoln("ServingConn")
	defer func() {
		c.close()
		log.WithError(err).WithField("connCount", decrementServeCount()).Infoln("ConnectionClosed")
		c.wg.Done()
	}()

	for {
		f, err := readFrame(c.Conn)
		if err != nil {
			return fmt.Errorf("read failed: %s", err)
		}
		log = log.WithField("received", f)

		ft, id, p := f.Disassemble()

		// If tp of tp_id exists, attempt to forward frame to tp.
		// delete tp on any failure.

		if tp, ok := c.getTp(id); ok {
			if err := tp.HandleFrame(f); err != nil {
				log.WithError(err).Warnf("Rejected [%s]: Transport closed.", ft)
			}
			continue
		}

		// if tp does not exist, frame should be 'REQUEST'.
		// otherwise, handle any unexpected frames accordingly.

		c.delTp(id) // rm tp in case closed tp is not fully removed.

		switch ft {
		case RequestType:
			c.wg.Add(1)
			go func(log *logrus.Entry) {
				defer c.wg.Done()
				initPK, err := c.handleRequestFrame(accept, id, p)
				if err != nil {
					log.WithField("remoteClient", initPK).WithError(err).Infoln("Rejected [REQUEST]")
					if isWriteError(err) || err == ErrClientClosed {
						err := c.Close()
						log.WithError(err).Warn("ClosingConnection")
					}
					return
				}
				log.WithField("remoteClient", initPK).Infoln("Accepted [REQUEST]")
			}(log)

		default:
			log.Infof("Ignored [%s]: No transport of given ID.", ft)
			if ft != CloseType {
				if err := writeCloseFrame(c.Conn, id, 0); err != nil {
					return err
				}
			}
		}
	}
}

// DialTransport dials a transport to remote dms_client.
func (c *ClientConnMap) DialTransport(ctx context.Context, clientPK cipher.PubKey) (*TransportMap, error) {
	tp, err := c.addTp(ctx, clientPK)
	if err != nil {
		return nil, err
	}
	if err := tp.WriteRequest(); err != nil {
		return nil, err
	}
	if err := tp.ReadAccept(ctx); err != nil {
		return nil, err
	}
	go tp.Serve()
	return tp, nil
}

// ClientOptionMap represents an optional argument for ClientMap.
type ClientOptionMap func(c *ClientMap) error

// SetLoggerMap sets the internal logger for ClientMap.
func SetLoggerMap(log *logging.Logger) ClientOptionMap {
	return func(c *ClientMap) error {
		if log == nil {
			return errors.New("nil logger set")
		}
		c.log = log
		return nil
	}
}

// ClientMap implements transport.Factory.
// It stores data in map instead of array.
type ClientMap struct {
	log *logging.Logger

	pk cipher.PubKey
	sk cipher.SecKey
	dc disc.APIClient

	conns map[cipher.PubKey]*ClientConnMap // conns with messaging servers. Key: pk of server
	mx    sync.RWMutex

	accept chan *TransportMap
	done   chan struct{}
	once   sync.Once
}

// NewClientMap creates a new ClientMap.
// It stores data in map instead of array.
func NewClientMap(pk cipher.PubKey, sk cipher.SecKey, dc disc.APIClient, opts ...ClientOptionMap) *ClientMap {
	c := &ClientMap{
		log:    logging.MustGetLogger("dmsg_client"),
		pk:     pk,
		sk:     sk,
		dc:     dc,
		conns:  make(map[cipher.PubKey]*ClientConnMap),
		accept: make(chan *TransportMap, AcceptBufferSize),
		done:   make(chan struct{}),
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			panic(err)
		}
	}
	return c
}

func (c *ClientMap) updateDiscEntry(ctx context.Context) error {
	srvPKs := make([]cipher.PubKey, 0, len(c.conns))
	for pk := range c.conns {
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

func (c *ClientMap) setConn(ctx context.Context, conn *ClientConnMap) {
	c.mx.Lock()
	c.conns[conn.remoteSrv] = conn
	if err := c.updateDiscEntry(ctx); err != nil {
		c.log.WithError(err).Warn("updateEntry: failed")
	}
	c.mx.Unlock()
}

func (c *ClientMap) delConn(ctx context.Context, pk cipher.PubKey) {
	c.mx.Lock()
	delete(c.conns, pk)
	if err := c.updateDiscEntry(ctx); err != nil {
		c.log.WithError(err).Warn("updateEntry: failed")
	}
	c.mx.Unlock()
}

func (c *ClientMap) getConn(pk cipher.PubKey) (*ClientConnMap, bool) {
	c.mx.RLock()
	l, ok := c.conns[pk]
	c.mx.RUnlock()
	return l, ok
}

func (c *ClientMap) connCount() int {
	c.mx.RLock()
	n := len(c.conns)
	c.mx.RUnlock()
	return n
}

// InitiateServerConnections initiates connections with dms_servers.
func (c *ClientMap) InitiateServerConnections(ctx context.Context, min int) error {
	if min == 0 {
		return nil
	}
	entries, err := c.findServerEntries(ctx)
	if err != nil {
		return err
	}
	c.log.Info("found dms_server entries:", entries)
	if err := c.findOrConnectToServers(ctx, entries, min); err != nil {
		return err
	}
	return nil
}

func (c *ClientMap) findServerEntries(ctx context.Context) ([]*disc.Entry, error) {
	for {
		entries, err := c.dc.AvailableServers(ctx)
		if err != nil || len(entries) == 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("dms_servers are not available: %s", err)
			default:
				retry := time.Second
				c.log.WithError(err).Warnf("no dms_servers found: trying again in %d second...", retry)
				time.Sleep(retry)
				continue
			}
		}
		return entries, nil
	}
}

func (c *ClientMap) findOrConnectToServers(ctx context.Context, entries []*disc.Entry, min int) error {
	for _, entry := range entries {
		_, err := c.findOrConnectToServer(ctx, entry.Static)
		if err != nil {
			c.log.Warnf("findOrConnectToServers: failed to find/connect to server %s: %s", entry.Static, err)
			continue
		}
		c.log.Infof("findOrConnectToServers: found/connected to server %s", entry.Static)
		if c.connCount() >= min {
			return nil
		}
	}
	return fmt.Errorf("findOrConnectToServers: all servers failed")
}

func (c *ClientMap) findOrConnectToServer(ctx context.Context, srvPK cipher.PubKey) (*ClientConnMap, error) {
	if conn, ok := c.getConn(srvPK); ok {
		return conn, nil
	}

	entry, err := c.dc.Entry(ctx, srvPK)
	if err != nil {
		return nil, err
	}
	if entry.Server == nil {
		return nil, errors.New("entry is of client instead of server")
	}

	tcpConn, err := net.Dial("tcp", entry.Server.Address)
	if err != nil {
		return nil, err
	}
	ns, err := noise.New(noise.HandshakeXK, noise.Config{
		LocalPK:   c.pk,
		LocalSK:   c.sk,
		RemotePK:  srvPK,
		Initiator: true,
	})
	if err != nil {
		return nil, err
	}
	nc, err := noise.WrapConn(tcpConn, ns, TransportHandshakeTimeout)
	if err != nil {
		return nil, err
	}

	conn := NewClientConnMap(c.log, nc, c.pk, srvPK)
	if err := conn.readOK(); err != nil {
		return nil, err
	}

	c.setConn(ctx, conn)

	go func() {
		err := conn.Serve(ctx, c.accept)
		conn.log.WithError(err).WithField("remoteServer", srvPK).Warn("connected with server closed")
		c.delConn(ctx, srvPK)

		// reconnect logic.
	retryServerConnect:
		select {
		case <-c.done:
		case <-ctx.Done():
		case <-time.After(clientReconnectInterval):
			conn.log.WithField("remoteServer", srvPK).Warn("Reconnecting")
			if _, err := c.findOrConnectToServer(ctx, srvPK); err != nil {
				conn.log.WithError(err).WithField("remoteServer", srvPK).Warn("ReconnectionFailed")
				goto retryServerConnect
			}
			conn.log.WithField("remoteServer", srvPK).Warn("ReconnectionSucceeded")
		}
	}()
	return conn, nil
}

// Accept accepts remotely-initiated tps.
func (c *ClientMap) Accept(ctx context.Context) (*TransportMap, error) {
	select {
	case tp, ok := <-c.accept:
		if !ok {
			return nil, ErrClientClosed
		}
		return tp, nil
	case <-c.done:
		return nil, ErrClientClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Dial dials a transport to remote dms_client.
func (c *ClientMap) Dial(ctx context.Context, remote cipher.PubKey) (*TransportMap, error) {
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
		conn, err := c.findOrConnectToServer(ctx, srvPK)
		if err != nil {
			c.log.WithError(err).Warn("failed to connect to server")
			continue
		}
		return conn.DialTransport(ctx, remote)
	}
	return nil, errors.New("failed to find dms_servers for given client pk")
}

// Local returns the local dms_client's public key.
func (c *ClientMap) Local() cipher.PubKey {
	return c.pk
}

// Type returns the transport type.
func (c *ClientMap) Type() string {
	return Type
}

// Close closes the dms_client and associated connections.
// TODO(evaninjin): proper error handling.
func (c *ClientMap) Close() error {
	c.once.Do(func() {
		close(c.done)
		for {
			select {
			case <-c.accept:
			default:
				close(c.accept)
				return
			}
		}
	})

	c.mx.Lock()
	for _, conn := range c.conns {
		_ = conn.Close()
	}
	c.conns = make(map[cipher.PubKey]*ClientConnMap)
	c.mx.Unlock()
	return nil
}
