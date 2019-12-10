package dmsg

import (
	"context"
	"fmt"
	"net"
	"sync"
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
	time.Sleep(time.Second * 2)

	// Helper functions.
	makePiper := func(dialer, listener *ClientEntity, port uint16) (net.Listener, nettest.MakePipe) {
		lis, err := listener.Listen(port)
		require.NoError(t, err)

		return lis, func() (c1, c2 net.Conn, stop func(), err error) {
			if c1, err = dialer.DialStream(context.TODO(), Addr{PK: listener.LocalPK(), Port: port}); err != nil {
				return
			}
			if c2, err = lis.Accept(); err != nil {
				return
			}
			stop = func() {
				//t.Log("Stopping pipe!")
				_ = c1.Close() //nolint:errcheck
				_ = c2.Close() //nolint:errcheck
			}
			return
		}
	}

	t.Run("test_listeners", func(t *testing.T) {
		const rounds = 3
		listeners := make([]net.Listener, 0, rounds*2)

		for port := uint16(1); port <= rounds; port++ {
			lis1, makePipe1 := makePiper(clientA, clientB, port)
			listeners = append(listeners, lis1)
			nettest.TestConn(t, makePipe1)

			lis2, makePipe2 := makePiper(clientB, clientA, port)
			listeners = append(listeners, lis2)
			nettest.TestConn(t, makePipe2)
		}

		// Closing logic.
		for _, lis := range listeners {
			require.NoError(t, lis.Close())
		}
	})

	t.Run("test_concurrent_listeners", func(t *testing.T) {
		const rounds = 5
		listeners := make([]net.Listener, 0, rounds*2)

		wg := new(sync.WaitGroup)
		wg.Add(rounds*2)

		for port := uint16(1); port <= rounds; port++ {
			lis1, makePipe1 := makePiper(clientA, clientB, port)
			listeners = append(listeners, lis1)
			go func(makePipe1 nettest.MakePipe) {
				nettest.TestConn(t, makePipe1)
				wg.Done()
			}(makePipe1)

			lis2, makePipe2 := makePiper(clientB, clientA, port)
			listeners = append(listeners, lis2)
			go func(makePipe2 nettest.MakePipe) {
				nettest.TestConn(t, makePipe2)
				wg.Done()
			}(makePipe2)
		}

		wg.Wait()
		fmt.Println("CLOSE LOGIC STARTED!")

		// Closing logic.
		for _, lis := range listeners {
			require.NoError(t, lis.Close())
		}
	})

	// Closing logic.
	require.NoError(t, clientB.Close())
	fmt.Println("CLOSE: client B stopped")
	require.NoError(t, clientA.Close())
	fmt.Println("CLOSE: client A stopped")
	require.NoError(t, srv.Close())
	fmt.Println("CLOSE: server stopped")
	require.NoError(t, <-chSrv)
}
