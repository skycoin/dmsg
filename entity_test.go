package dmsg

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"

	"github.com/SkycoinProject/dmsg/disc"
)

func TestNewClientEntity(t *testing.T) {
	// Prepare mock discovery.
	dc := disc.NewMock()

	// Prepare dmsg server.
	pkSrv, skSrv := GenKeyPair(t, "server")
	srv := NewServer(pkSrv, skSrv, dc)
	srv.SetLogger(logging.MustGetLogger("server"))
	lisSrv, err := nettest.NewLocalListener("tcp")
	require.NoError(t, err)

	// Serve dmsg server.
	chSrv := make(chan error, 1)
	go func() { chSrv <- srv.Serve(lisSrv, "") }() //nolint:errcheck

	// Prepare and serve dmsg client A.
	pkA, skA := GenKeyPair(t, "client A")
	clientA := NewClient(pkA, skA, dc, DefaultConfig())
	clientA.SetLogger(logging.MustGetLogger("client_A"))
	go clientA.Serve()

	// Prepare and serve dmsg client B.
	pkB, skB := GenKeyPair(t, "client B")
	clientB := NewClient(pkB, skB, dc, DefaultConfig())
	clientB.SetLogger(logging.MustGetLogger("client_B"))
	go clientB.Serve()

	// Ensure all entities are registered in discovery before continuing.
	time.Sleep(time.Second)

	// Make client A start listening.
	portA := uint16(80)
	lisA, err := clientA.Listen(portA)
	require.NoError(t, err)

	// Test dial and accept streams between client A and B.
	nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
		if c1, err = clientB.DialStream(context.TODO(), Addr{PK: pkA, Port: portA}); err != nil {
			return
		}
		if c2, err = lisA.AcceptStream(); err != nil {
			return
		}
		stop = func() {
			_ = c1.Close()
			_ = c2.Close()
		}
		return
	})

	// Closing logic.
	require.NoError(t, lisA.Close())
	require.NoError(t, clientB.Close())
	require.NoError(t, clientA.Close())
	require.NoError(t, srv.Close())
	require.NoError(t, <-chSrv)
}
