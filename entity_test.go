package dmsg

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"

	"github.com/SkycoinProject/dmsg/disc"
)

func TestNewClientEntity(t *testing.T) {
	dc := disc.NewMock()

	sPK, sSK := GenKeyPair(t, "server")
	srv := NewServer(sPK, sSK, dc)
	srv.SetLogger(logging.MustGetLogger("server"))
	srvL, err := nettest.NewLocalListener("tcp")
	require.NoError(t, err)

	go func() { _ = srv.Serve(context.TODO(), srvL, "") }()

	//time.Sleep(time.Second)

	aPK, aSK := GenKeyPair(t, "client A")
	a := NewClient(aPK, aSK, dc, DefaultConfig())
	a.SetLogger(logging.MustGetLogger("client_A"))
	go a.Serve()

	bPK, bSK := GenKeyPair(t, "client B")
	b := NewClient(bPK, bSK, dc, DefaultConfig())
	b.SetLogger(logging.MustGetLogger("client_B"))
	go b.Serve()

	time.Sleep(time.Second)

	aPort := uint16(80)
	aL, err := a.Listen(aPort)
	require.NoError(t, err)

	bStr, err := b.DialStream(context.TODO(), Addr{PK: aPK, Port: aPort})
	require.NoError(t, err)
	aStr, err := aL.AcceptStream()
	require.NoError(t, err)

	fmt.Println("stream A:", aStr.StreamID())
	fmt.Println("stream B:", bStr.StreamID())

	nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
		if c1, err = b.DialStream(context.TODO(), Addr{PK: aPK, Port: aPort}); err != nil {
			return
		}
		if c2, err = aL.AcceptStream(); err != nil {
			return
		}
		stop = func() {
			_ = c1.Close()
			_ = c2.Close()
		}
		return
	})
}
