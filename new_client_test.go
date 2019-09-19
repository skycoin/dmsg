// +build !no_ci

package dmsg

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
)

// TODO: update comments mentioning a & b
// Given two client instances (a & b) and a server instance (s),
// Client b should be able to dial a transport with client b
// Data should be sent and delivered successfully via the transport.
// TODO: fix this.
func TestNewClient(t *testing.T) {
	srvPK, srvSK := cipher.GenerateKeyPair()
	sAddr := "127.0.0.1:8081"

	const tpCount = 10

	dc := disc.NewMock()

	l, err := net.Listen("tcp", sAddr)
	require.NoError(t, err)

	srv, err := NewServer(srvPK, srvSK, "", l, dc)
	require.NoError(t, err)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve()
		close(serveErr)
	}()

	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	errCh := make(chan error, 1)
	go func() {
		defer wg.Done()

		for i := 0; i < tpCount; i++ {
			initiatorTp, responderTp := dial(t, initiator, responder, port+uint16(i), noDelay)

			for j := 0; j < msgCount; j++ {
				pay := []byte(fmt.Sprintf("This is message %d!", j))

				if _, err := responderTp.Write(pay); err != nil {
					errCh <- err
					return
				}

				if _, err := initiatorTp.Read(pay); err != nil {
					errCh <- err
					return
				}
			}

			if err := closeClosers(responderTp, initiatorTp); err != nil {
				errCh <- err
				return
			}
		}

		errCh <- nil
	}()

	for i := 0; i < tpCount; i++ {
		initiatorTp, responderTp := dial(t, initiator, responder, port+tpCount+uint16(i), noDelay)

		for j := 0; j < msgCount; j++ {
			pay := []byte(fmt.Sprintf("This is message %d!", j))

			n, err := responderTp.Write(pay)
			require.NoError(t, err)
			require.Equal(t, len(pay), n)

			got := make([]byte, len(pay))
			n, err = initiatorTp.Read(got)
			require.Equal(t, len(pay), n)
			require.NoError(t, err)
			require.Equal(t, pay, got)
		}

		// Close TPs
		require.NoError(t, closeClosers(responderTp, initiatorTp))
	}

	wg.Wait()
	assert.NoError(t, <-errCh)

	assert.NoError(t, srv.Close())
	assert.NoError(t, errWithTimeout(serveErr))
}
