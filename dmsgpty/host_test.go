package dmsgpty

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/dmsgtest"
)

func TestHost(t *testing.T) {

	// Prepare dmsg env.
	env := dmsgtest.NewEnv(t, dmsgtest.DefaultTimeout)
	require.NoError(t, env.Startup(1, 3, nil))
	defer env.Shutdown()

	dClients := env.AllClients()
	var (
		dcA = dClients[0]
		dcB = dClients[1]
	)

	// Prepare whitelist.
	wl, rmWl := tempWhitelist(t)
	defer rmWl()
	require.NoError(t, wl.Add(dcA.LocalPK()))
	require.NoError(t, wl.Add(dcB.LocalPK()))

	t.Run("serveConn_whitelist", func(t *testing.T) {
		connH, connC := net.Pipe()
		defer func() {
			require.NoError(t, connH.Close())
			require.NoError(t, connC.Close())
		}()

		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		host := NewHost(dcA, wl)
		go host.serveConn(ctx, logging.MustGetLogger("host_conn"), connH)

		wlC, err := NewWhitelistClient(connC)
		require.NoError(t, err)

		for i := 0; i < 10; i++ {
			pks, err := wlC.ViewWhitelist()
			require.NoError(t, err)
			require.Len(t, pks, 2)

			pk, _ := cipher.GenerateKeyPair()
			require.NoError(t, wlC.WhitelistAdd(pk), i)
			require.NoError(t, wlC.WhitelistRemove(pk), i)
		}
	})

	t.Run("serveConn_pty", func(t *testing.T) {
		connH, connC := net.Pipe()
		defer func() {
			require.NoError(t, connH.Close())
			require.NoError(t, connC.Close())
		}()

		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		host := NewHost(dcA, wl)
		go host.serveConn(ctx, logging.MustGetLogger("host_conn"), connH)

		ptyC, err := NewPtyClient(connC)
		require.NoError(t, err)

		msg := "Hello world!"
		require.NoError(t, ptyC.Start("echo", msg))

		readB := make([]byte, len(msg))
		n, err := io.ReadFull(ptyC, readB)
		require.NoError(t, err)
		require.Equal(t, len(readB), n)
		require.Equal(t, msg, string(readB))
	})

	t.Run("serveConn_proxy", func(t *testing.T) {
		connB, connCLI := net.Pipe()
		defer func() {
			require.NoError(t, connB.Close())
			require.NoError(t, connCLI.Close())
		}()

		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		const port = uint16(22)

		hostA := NewHost(dcA, wl)
		errA := make(chan error, 1)
		go func() {
			errA <- hostA.ListenAndServe(ctx, port)
			close(errA)
		}()

		hostB := NewHost(dcB, wl)
		go hostB.serveConn(ctx, logging.MustGetLogger("hostB_conn"), connB)

		ptyB, err := NewPtyProxyClient(connCLI, dcA.LocalPK(), port)
		require.NoError(t, err)

		msg := "Hello world!"
		require.NoError(t, ptyB.Start("echo", msg))

		readB := make([]byte, len(msg))
		n, err := io.ReadFull(ptyB, readB)
		require.NoError(t, err)
		require.Equal(t, len(readB), n)
		require.Equal(t, msg, string(readB))
	})
}

func tempWhitelist(t *testing.T) (Whitelist, func()) {
	f, err := ioutil.TempFile(os.TempDir(), "")
	require.NoError(t, err)

	fName := f.Name()
	require.NoError(t, f.Close())

	wl, err := NewJSONFileWhiteList(fName)
	require.NoError(t, err)

	return wl, func() {
		require.NoError(t, os.Remove(fName))
	}
}
