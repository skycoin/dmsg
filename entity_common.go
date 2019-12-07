package dmsg

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/netutil"
)

type EntityCommon struct {
	pk cipher.PubKey
	sk cipher.SecKey
	dc disc.APIClient

	ss map[cipher.PubKey]*SessionCommon
	mx *sync.Mutex

	log logrus.FieldLogger

	setSessionCallback func(ctx context.Context) error
	delSessionCallback func(ctx context.Context) error
}

func (c *EntityCommon) init(pk cipher.PubKey, sk cipher.SecKey, dc disc.APIClient, log logrus.FieldLogger) {
	c.pk = pk
	c.sk = sk
	c.dc = dc
	c.ss = make(map[cipher.PubKey]*SessionCommon)
	c.mx = new(sync.Mutex)
	c.log = log
}

func (c *EntityCommon) LocalPK() cipher.PubKey { return c.pk }

func (c *EntityCommon) SetLogger(log logrus.FieldLogger) { c.log = log }

func (c *EntityCommon) session(pk cipher.PubKey) (*SessionCommon, bool) {
	c.mx.Lock()
	dSes, ok := c.ss[pk]
	c.mx.Unlock()
	return dSes, ok
}

func (c *EntityCommon) ServerSession(pk cipher.PubKey) (ServerSession, bool) {
	ses, ok := c.session(pk)
	return ServerSession{SessionCommon: ses}, ok
}

func (c *EntityCommon) ClientSession(porter *netutil.Porter, pk cipher.PubKey) (ClientSession, bool) {
	ses, ok := c.session(pk)
	return ClientSession{SessionCommon: ses, porter: porter}, ok
}

func (c *EntityCommon) SessionCount() int {
	c.mx.Lock()
	n := len(c.ss)
	c.mx.Unlock()
	return n
}

func (c *EntityCommon) setSession(ctx context.Context, dSes *SessionCommon) {
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
// TODO(evanlinjin): THis should be in ClientEntity.
func (c *EntityCommon) initiateAndServeSession(ctx context.Context, porter *netutil.Porter, entry *disc.Entry, sesCh chan<- ClientSession) error {

	conn, err := net.Dial("tcp", entry.Server.Address)
	if err != nil {
		return err
	}
	dSes, err := makeClientSession(c, porter, conn, entry.Static)
	if err != nil {
		return err
	}

	c.setSession(ctx, dSes.SessionCommon)
	defer func() {
		c.delSession(ctx, dSes.RemotePK())
		_ = dSes.Close() //nolint:errcheck
	}()

	notifyOfSession(ctx, sesCh, dSes)
	for {
		if _, err := dSes.acceptStream(); err != nil {
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

func notifyOfSession(ctx context.Context, sesCh chan<- ClientSession, dSes ClientSession) {
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
