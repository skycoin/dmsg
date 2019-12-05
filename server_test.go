package dmsg

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"

	"github.com/SkycoinProject/dmsg/disc"
)

func TestNewServer(t *testing.T) {
	dmsgDisc := disc.NewMock()

	sPK, sSK := GenKeyPair(t, "server")

	srvL, err := nettest.NewLocalListener("tcp")
	require.NoError(t, err)

	srv, err := NewServer(sPK, sSK, "", srvL, dmsgDisc)
	require.NoError(t, err)
	srv.SetLogger(logging.MustGetLogger("server"))

	go func() {
		_ = srv.Serve()
		panic("no") // TODO: remove this.
	}()
	time.Sleep(time.Second * 2)

	aPK, aSK := GenKeyPair(t, "client A")
	a := NewClient(aPK, aSK, dmsgDisc, SetLogger(logging.MustGetLogger("client_A")))
	require.NoError(t, a.InitiateServerConnections(context.TODO(), 1))

	bPK, bSK := GenKeyPair(t, "client B")
	b := NewClient(bPK, bSK, dmsgDisc, SetLogger(logging.MustGetLogger("client_B")))
	require.NoError(t, b.InitiateServerConnections(context.TODO(), 1))

	aPort := uint16(80)
	aL, err := a.Listen(aPort)
	require.NoError(t, err)

	bStr, err := b.DialStream(context.TODO(), aPK, aPort)
	require.NoError(t, err)

	aStr, err := aL.AcceptStream()
	require.NoError(t, err)

	fmt.Println(aStr.StreamID())
	fmt.Println(bStr.StreamID())
}
