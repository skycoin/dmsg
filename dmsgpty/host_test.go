package dmsgpty

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"golang.org/x/net/nettest"

	"github.com/SkycoinProject/dmsg"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/dmsgtest"
)

func TestHost(t *testing.T) {
	const port = uint16(22)

	// Prepare dmsg env.
	env := dmsgtest.NewEnv(t, dmsgtest.DefaultTimeout)
	require.NoError(t, env.Startup(1, 2, nil))

	dcA := env.AllClients()[0]
	dcB := env.AllClients()[1]

	// Prepare whitelist.
	wl, delWhitelist := tempWhitelist(t)
	require.NoError(t, wl.Add(dcA.LocalPK()))
	require.NoError(t, wl.Add(dcB.LocalPK()))

	t.Run("serveConn_whitelist", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		connH, connC := net.Pipe()

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

		// Closing logic.
		cancel()
		require.NoError(t, connH.Close())
		require.NoError(t, connC.Close())
	})

	t.Run("serveConn_pty", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		connH, connC := net.Pipe()

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

		// Closing logic.
		cancel()
		require.NoError(t, connH.Close())
		require.NoError(t, connC.Close())
	})

	t.Run("serveConn_proxy", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		connB, connCLI := net.Pipe()

		hostA := NewHost(dcA, wl)
		errA := make(chan error, 1)
		go func() {
			errA <- hostA.ListenAndServe(ctx, port)
			close(errA)
		}()

		hostB := NewHost(dcB, wl)
		go hostB.serveConn(ctx, logging.MustGetLogger("hostB_conn"), connB)

		ptyB, err := NewProxyClient(connCLI, dcA.LocalPK(), port)
		require.NoError(t, err)

		msg := "Hello world!"
		require.NoError(t, ptyB.Start("echo", msg))

		readB := make([]byte, len(msg))
		n, err := io.ReadFull(ptyB, readB)
		require.NoError(t, err)
		require.Equal(t, len(readB), n)
		require.Equal(t, msg, string(readB))

		// Closing logic.
		cancel()
		require.EqualError(t, <-errA, dmsg.ErrEntityClosed.Error())
		require.NoError(t, connB.Close())
		require.NoError(t, connCLI.Close())
	})

	t.Run("ServeCLI", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		cliL, err := nettest.NewLocalListener("tcp")
		require.NoError(t, err)

		hostA := NewHost(dcA, wl)
		errA := make(chan error, 1)
		go func() {
			errA <- hostA.ListenAndServe(ctx, port)
			close(errA)
		}()

		hostB := NewHost(dcB, wl)
		errB := make(chan error, 1)
		go func() {
			errB <- hostB.ServeCLI(ctx, cliL)
			close(errB)
		}()

		cliB := &CLI{
			Net:  cliL.Addr().Network(),
			Addr: cliL.Addr().String(),
		}

		t.Run("CLI.WhitelistClient", func(t *testing.T) {
			wlC, err := cliB.WhitelistClient()
			require.NoError(t, err)
			pks, err := wlC.ViewWhitelist()
			require.NoError(t, err)
			require.Len(t, pks, 2)
		})

		t.Run("CLI.StartLocalPty", func(t *testing.T) {
			// TODO(evanlinjin): Complete.
			//msg := "Hello World!"
			//require.NoError(t, cliB.StartLocalPty(ctx, "echo", msg))
		})

		t.Run("CLI.StartRemotePty", func(t *testing.T) {
			// TODO(evanlinjin): Complete.
		})

		// Closing logic.
		cancel()
		require.NoError(t, cliL.Close())
		require.EqualError(t, <-errA, dmsg.ErrEntityClosed.Error())
		require.Error(t, <-errB)
	})

	// Closing logic.
	delWhitelist()
	env.Shutdown()
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
