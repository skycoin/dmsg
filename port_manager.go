package dmsg

import (
	"context"
	"sync"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/netutil"
)

// PortManager manages ports of nodes.
type PortManager struct {
	p *netutil.Porter
}

func newPortManager() *PortManager {
	return &PortManager{
		p: netutil.NewPorter(netutil.PorterMinEphemeral),
	}
}

// Listener returns a listener assigned to a given port.
func (pm *PortManager) Listener(port uint16) (*Listener, bool) {
	v, ok := pm.p.PortValue(port)
	if !ok {
		return nil, false
	}
	l, ok := v.(*Listener)
	return l, ok
}

// NewListener assigns listener to port if port is available.
func (pm *PortManager) NewListener(pk cipher.PubKey, port uint16) (*Listener, bool) {
	l := newListener(pk, port)
	ok, clear := pm.p.Reserve(port, l)
	if !ok {
		return nil, false
	}
	l.AddCloseCallback(clear)
	return l, true
}

func (pm *PortManager) ReserveEphemeral(ctx context.Context) (uint16, func(), error) {
	return pm.p.ReserveEphemeral(ctx, nil)
}

func (pm *PortManager) Close() error {
	wg := new(sync.WaitGroup)
	pm.p.RangePortValues(func(_ uint16, v interface{}) (next bool) {
		l, ok := v.(*Listener)
		if ok {
			wg.Add(1)
			go func() {
				l.close()
				wg.Done()
			}()
		}
		return true
	})
	wg.Wait()
	return nil
}
