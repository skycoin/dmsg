package dmsgpty

// TODO(evanlinjin): fix failing tests
/*
func TestHost(t *testing.T) {
	const port = uint16(22)
	defaultConf := dmsg.Config{MinSessions: 2}

	// Prepare dmsg env.
	env := dmsgtest.NewEnv(t, dmsgtest.DefaultTimeout)
	require.NoError(t, env.Startup(2, 2, 1, &defaultConf))

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
		hMux := cliEndpoints(host)
		go host.serveConn(ctx, logging.MustGetLogger("host_conn"), &hMux, connH)

		wlC, err := NewWhitelistClient(connC)
		require.NoError(t, err)

		checkWhitelist(t, wlC, 2, 10)

		// Closing logic.
		cancel()
		require.NoError(t, connH.Close())
		require.NoError(t, connC.Close())
	})

	t.Run("serveConn_pty", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		connH, connC := net.Pipe()

		host := NewHost(dcA, wl)
		hMux := cliEndpoints(host)
		go host.serveConn(ctx, logging.MustGetLogger("host_conn"), &hMux, connH)

		ptyC, err := NewPtyClient(connC)
		require.NoError(t, err)

		checkPty(t, ptyC, "Hello World!")

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
		hBMux := cliEndpoints(hostB)
		go hostB.serveConn(ctx, logging.MustGetLogger("hostB_conn"), &hBMux, connB)

		ptyB, err := NewProxyClient(connCLI, dcA.LocalPK(), port)
		require.NoError(t, err)

		checkPty(t, ptyB, "Hello World!")

		// Closing logic.
		cancel()
		require.NoError(t, <-errA)
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
			Log:  logging.MustGetLogger("dmsgpty-cli"),
			Net:  cliL.Addr().Network(),
			Addr: cliL.Addr().String(),
		}

		t.Run("endpoint_whitelist", func(t *testing.T) {
			wlC, err := cliB.WhitelistClient()
			require.NoError(t, err)

			checkWhitelist(t, wlC, 2, 10)
		})

		t.Run("endpoint_pty", func(t *testing.T) {
			conn, err := cliB.prepareConn()
			require.NoError(t, err)

			ptyB, err := NewPtyClient(conn)
			require.NoError(t, err)

			for i := 20; i < 100; i += 10 {
				checkPty(t, ptyB, fmt.Sprintf("Hello World! %d", i))
			}

			require.NoError(t, conn.Close())
		})

		t.Run("endpoint_proxy", func(t *testing.T) {
			conn, err := cliB.prepareConn()
			require.NoError(t, err)

			ptyB, err := NewProxyClient(conn, dcA.LocalPK(), port)
			require.NoError(t, err)

			for i := 20; i < 100; i += 10 {
				checkPty(t, ptyB, fmt.Sprintf("Hello World! %d", i))
			}

			require.NoError(t, conn.Close())
		})

		// A non-whitelisted host should have no access to hostA's pty.
		t.Run("no_access", func(t *testing.T) {
			dcC, err := env.NewClient(&defaultConf)
			require.NoError(t, err)

			lisC, err := nettest.NewLocalListener("tcp")
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(ctx)
			hostC := NewHost(dcC, wl)
			cErr := make(chan error, 1)
			go func() {
				cErr <- hostC.ServeCLI(ctx, lisC)
				close(cErr)
			}()

			cliC := CLI{
				Log:  logging.MustGetLogger("cli_c"),
				Net:  lisC.Addr().Network(),
				Addr: lisC.Addr().String(),
			}

			conn, err := cliC.prepareConn()
			require.NoError(t, err)

			_, err = NewProxyClient(conn, dcA.LocalPK(), port)
			require.Error(t, err)

			// Closing logic.
			cancel()
			require.NoError(t, <-cErr)
		})

		// Closing logic.
		cancel()
		require.NoError(t, <-errA)
		require.NoError(t, <-errB)
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

func checkPty(t *testing.T, ptyC *PtyClient, msg string) {
	require.NoError(t, ptyC.Start("echo", msg))

	readB := make([]byte, len(msg))
	n, err := io.ReadFull(ptyC, readB)
	require.NoError(t, err)
	require.Equal(t, len(readB), n)
	require.Equal(t, msg, string(readB))

	require.NoError(t, ptyC.Stop())
}

func checkWhitelist(t *testing.T, wlC *WhitelistClient, initN, rounds int) {
	pks, err := wlC.ViewWhitelist()
	require.NoError(t, err)
	require.Len(t, pks, initN)

	newPKS := make([]cipher.PubKey, rounds)
	for i := 0; i < rounds; i++ {
		pk, _ := cipher.GenerateKeyPair()
		require.NoError(t, wlC.WhitelistAdd(pk), i)
		newPKS[i] = pk

		pks, err := wlC.ViewWhitelist()
		require.NoError(t, err)
		require.Len(t, pks, initN+i+1)
	}
	for i, newPK := range newPKS {
		require.NoError(t, wlC.WhitelistRemove(newPK))

		pks, err := wlC.ViewWhitelist()
		require.NoError(t, err)
		require.Len(t, pks, initN+len(newPKS)-i-1)
	}
}
*/
