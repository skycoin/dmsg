package dmsg

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/noise"
)

const (
	responderName = "responder"
	initiatorName = "initiator"
	message       = "Hello there!"
	msgCount      = 100
	bufSize       = 5
	port          = uint16(1)
)

func TestMain(m *testing.M) {
	loggingLevel, ok := os.LookupEnv("TEST_LOGGING_LEVEL")
	if ok {
		lvl, err := logging.LevelFromString(loggingLevel)
		if err != nil {
			log.Fatal(err)
		}
		logging.SetLevel(lvl)
	} else {
		logging.Disable()
	}

	os.Exit(m.Run())
}

// TestServerConn_AddNext ensures that `nextConns` for the remote client is being filled correctly.
func TestServerConn_AddNext(t *testing.T) {
	type want struct {
		id      uint16
		wantErr bool
	}

	pk, _ := cipher.GenerateKeyPair()

	fullNextConns := make(map[uint16]*NextConn)
	fullNextConns[1] = &NextConn{}
	for i := uint16(3); i != 1; i += 2 {
		fullNextConns[i] = &NextConn{}
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), smallDelay)
	defer cancel()

	cases := []struct {
		name string
		conn *ServerConn
		ctx  context.Context
		want want
	}{
		{
			name: "ok",
			conn: &ServerConn{
				remoteClient: pk,
				log:          logging.MustGetLogger("ServerConn"),
				nextRespID:   1,
				nextConns:    map[uint16]*NextConn{},
			},
			ctx: context.Background(),
			want: want{
				id: 1,
			},
		},
		{
			name: "ok, skip 1",
			conn: &ServerConn{
				remoteClient: pk,
				log:          logging.MustGetLogger("ServerConn"),
				nextRespID:   1,
				nextConns: map[uint16]*NextConn{
					1: {},
				},
			},
			ctx: context.Background(),
			want: want{
				id: 3,
			},
		},
		{
			name: "fail - timed out",
			conn: &ServerConn{
				remoteClient: pk,
				log:          logging.MustGetLogger("ServerConn"),
				nextRespID:   1,
				nextConns:    fullNextConns,
			},
			ctx: timeoutCtx,
			want: want{
				wantErr: true,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			id, err := tc.conn.addNext(tc.ctx, &NextConn{})

			if tc.want.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if err != nil {
				return
			}

			require.Equal(t, tc.want.id, id)
		})
	}

	// concurrent
	connsCount := 50

	serverConn := &ServerConn{
		log:          logging.MustGetLogger("ServerConn"),
		remoteClient: pk,
		nextRespID:   1,
		nextConns:    map[uint16]*NextConn{},
	}

	var wg sync.WaitGroup
	wg.Add(connsCount)
	for i := 0; i < connsCount; i++ {
		go func() {
			defer wg.Done()

			_, err := serverConn.addNext(context.Background(), &NextConn{})
			require.NoError(t, err)
		}()
	}

	wg.Wait()

	for i := uint16(1); i < uint16(connsCount*2); i += 2 {
		_, ok := serverConn.getNext(i)
		require.True(t, ok)
	}

	for i := uint16(connsCount*2 + 1); i != 1; i += 2 {
		_, ok := serverConn.getNext(i)
		require.False(t, ok)
	}
}

// TestNewServer ensures Server starts and quits with no error.
func TestNewServer(t *testing.T) {
	srvPK, srvSK := cipher.GenerateKeyPair()
	dc := disc.NewMock()

	l, err := net.Listen("tcp", "")
	require.NoError(t, err)

	// When calling 'NewServer', if the provided net.Listener is already a noise.Listener,
	// An error should be returned.
	t.Run("Already wrapped listener fails", func(t *testing.T) {
		wrappedL := noise.WrapListener(l, srvPK, srvSK, false, noise.HandshakeXK)
		s, err := NewServer(srvPK, srvSK, "", wrappedL, dc)
		assert.Equal(t, ErrListenerAlreadyWrappedToNoise, err)
		assert.Nil(t, s)
	})

	t.Run("should_start_and_stop_okay", func(t *testing.T) {
		s, err := NewServer(srvPK, srvSK, "", l, dc)
		require.NoError(t, err)

		var serveErr error
		serveDone := make(chan struct{})
		go func() {
			serveErr = s.Serve()
			close(serveDone)
		}()

		time.Sleep(smallDelay)

		require.NoError(t, s.Close())

		<-serveDone
		require.NoError(t, serveErr)
	})
}

// TestServer_Serve ensures that Server processes request frames and
// instantiates transports properly.
func TestServer_Serve(t *testing.T) {
	t.Run("Transport establishes", func(t *testing.T) {
		testServerTransportEstablishment(t)
	})

	t.Run("Transport establishes concurrently", func(t *testing.T) {
		testServerConcurrentTransportEstablishment(t)
	})

	t.Run("Failed accepts do not result in hang", func(t *testing.T) {
		testServerFailedAccepts(t)
	})

	t.Run("Sent/received message is consistent", func(t *testing.T) {
		testServerMessageConsistency(t)
	})

	t.Run("Capped transport buffer does not result in hang", func(t *testing.T) {
		testServerCappedTransport(t)
	})

	t.Run("Self dialing works", func(t *testing.T) {
		testServerSelfDialing(t)
	})

	t.Run("Server disconnection closes transports", func(t *testing.T) {
		testServerDisconnection(t)
	})

	t.Run("Reconnection to server succeeds", func(t *testing.T) {
		t.Parallel()

		t.Run("Same address", func(t *testing.T) {
			testServerReconnection(t, false)
		})

		t.Run("Random address", func(t *testing.T) {
			testServerReconnection(t, true)
		})
	})
}

func testServerDisconnection(t *testing.T) {
	t.Parallel()

	dc := disc.NewMock()
	srv, srvErrCh, err := createServer(dc)
	require.NoError(t, err)

	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)
	initConn, respConns := dial(t, initiator, responder, port, noDelay)
	testTransportMessaging(t, initConn, respConns)

	require.NoError(t, srv.Close())
	require.NoError(t, errWithTimeout(srvErrCh))

	time.Sleep(smallDelay)

	require.True(t, initConn.(*Transport).IsClosed())
	require.True(t, respConns.(*Transport).IsClosed())
}

func testServerSelfDialing(t *testing.T) {
	t.Parallel()

	dc := disc.NewMock()
	srv, srvErrCh, err := createServer(dc)
	require.NoError(t, err)

	client := createClient(t, dc, "client")
	selfWrTp, selfRdTp := dial(t, client, client, port, noDelay)
	// try to write/read message to/from self
	testTransportMessaging(t, selfWrTp, selfRdTp)
	require.NoError(t, closeClosers(selfRdTp, selfWrTp, client))

	assert.NoError(t, srv.Close())
	assert.NoError(t, errWithTimeout(srvErrCh))
}

func testTransportMessaging(t *testing.T, init, resp io.ReadWriter) {
	for i := 0; i < msgCount; i++ {
		_, err := init.Write([]byte(message))
		require.NoError(t, err) // TODO: Sometimes this returns error: "io: read/write on closed pipe"

		recvBuf := make([]byte, bufSize)
		for i := 0; i < len(message); i += bufSize {
			_, err := resp.Read(recvBuf)
			require.NoError(t, err)
		}
	}
}

func testServerCappedTransport(t *testing.T) {
	// TODO(evanlinjin): I've disabled this as it was causing writes to closed connections.
	//t.Parallel()

	dc := disc.NewMock()
	srv, srvErrCh, err := createServer(dc)
	require.NoError(t, err)

	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)
	// responder calls initiator
	responderWrConn, _ := dial(t, responder, initiator, port, noDelay)
	msg := []byte(message)
	// exact iterations to fill the receiving buffer and hang `Write`
	iterationsToDo := tpBufCap/len(msg) + 1
	// fill the buffer, but no block yet
	for i := 0; i < iterationsToDo-1; i++ {
		_, err := responderWrConn.Write(msg)
		require.NoError(t, err)
	}
	// block on `Write`
	var blockedWriteErr error
	blockedWriteDone := make(chan struct{})
	go func() {
		_, blockedWriteErr = responderWrConn.Write(msg)
		close(blockedWriteDone)
	}()
	// wait till it's definitely blocked
	initiatorWrConn, responderRdConn := dial(t, initiator, responder, port, smallDelay)
	// try to write/read message via the new transports
	for i := 0; i < msgCount; i++ {
		_, err := initiatorWrConn.Write(msg)
		require.NoError(t, err)

		recBuff := make([]byte, len(msg))
		_, err = responderRdConn.Read(recBuff)
		require.NoError(t, err)

		require.Equal(t, recBuff, msg)
	}
	err = responderWrConn.Close()
	require.NoError(t, err)
	<-blockedWriteDone
	require.Error(t, blockedWriteErr)
	require.NoError(t, closeClosers(initiatorWrConn, responderRdConn, responder, initiator))

	assert.NoError(t, srv.Close())
	assert.NoError(t, errWithTimeout(srvErrCh))
}

func testServerFailedAccepts(t *testing.T) {
	t.Parallel()

	dc := disc.NewMock()
	srv, srvErrCh, err := createServer(dc)
	require.NoError(t, err)

	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)
	initiatorConn, responderConn := dial(t, initiator, responder, port, noDelay)
	readWriteStop := make(chan struct{})
	readWriteDone := make(chan struct{})
	var readErr, writeErr error
	go func() {
		// read/write to/from connection until the stop signal arrives
		for {
			select {
			case <-readWriteStop:
				close(readWriteDone)
				return
			default:
				if _, writeErr = initiatorConn.Write([]byte(message)); writeErr != nil {
					close(readWriteDone)
					return
				}
				if _, readErr = responderConn.Read([]byte(message)); readErr != nil {
					close(readWriteDone)
					return
				}
			}
		}
	}()

	// Waiting for error on Dial which happens when the buffer is being filled with the incoming dials.
	// Call Dial in a loop without any Accepts until an error occurs.
	for {
		ctx := context.Background()
		if _, err = responder.Dial(ctx, initiator.pk, port); err != nil {
			break
		}
	}
	// must be error
	require.Error(t, err)
	// the same as above, connection is created by another client
	for {
		ctx := context.Background()
		if _, err = initiator.Dial(ctx, responder.pk, port); err != nil {
			break
		}
	}
	// must be error
	require.Error(t, err)
	// wait more time to ensure that the initially created connection works
	time.Sleep(smallDelay)
	require.NoError(t, closeClosers(responderConn, initiatorConn))
	// stop reading/writing goroutines
	close(readWriteStop)
	<-readWriteDone
	// check that the initial connection had been working properly all the time
	// if any error, it must be `io.EOF` for reader
	if readErr != io.EOF {
		require.NoError(t, readErr)
	}
	// if any error, it must be `io.ErrClosedPipe` for writer
	if writeErr != io.ErrClosedPipe {
		require.NoError(t, writeErr)
	}
	require.NoError(t, closeClosers(responder, initiator))

	assert.NoError(t, srv.Close())
	assert.NoError(t, errWithTimeout(srvErrCh))
}

// connect two clients, establish transport, check if there are
// two ServerConn's and that both conn's `nextConn` is filled correctly
func testServerTransportEstablishment(t *testing.T) {
	t.Parallel()

	dc := disc.NewMock()
	srv, srvErrCh, err := createServer(dc)
	require.NoError(t, err)

	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)
	initConn, respConn := dial(t, initiator, responder, port, noDelay)
	// must be 2 ServerConn's
	checkConnCount(t, smallDelay, 2, srv)
	// must have ServerConn for A
	responderServerConn, ok := srv.getConn(responder.pk)
	require.True(t, ok)
	require.Equal(t, responder.pk, responderServerConn.PK())
	// must have ServerConn for B
	initiatorServerConn, ok := srv.getConn(initiator.pk)
	require.True(t, ok)
	require.Equal(t, initiator.pk, initiatorServerConn.PK())
	// must have a ClientConn
	responderClientConn, ok := responder.getConn(srv.pk)
	require.True(t, ok)
	require.Equal(t, srv.pk, responderClientConn.RemotePK())
	// must have a ClientConn
	initiatorClientConn, ok := initiator.getConn(srv.pk)
	require.True(t, ok)
	require.Equal(t, srv.pk, initiatorClientConn.RemotePK())
	// check whether nextConn's contents are as must be
	nextInitID := getNextInitID(initiatorClientConn)
	initiatorNextConn, ok := initiatorServerConn.getNext(nextInitID - 2)
	require.True(t, ok)
	nextRespID := getNextRespID(responderServerConn)
	require.Equal(t, initiatorNextConn.id, nextRespID-2)
	// check whether nextConn's contents are as must be
	nextRespID = getNextRespID(responderServerConn)
	responderNextConn, ok := responderServerConn.getNext(nextRespID - 2)
	require.True(t, ok)
	nextInitID = getNextInitID(initiatorClientConn)
	require.Equal(t, responderNextConn.id, nextInitID-2)
	require.NoError(t, closeClosers(respConn, initConn, responder, initiator))
	checkConnCount(t, smallDelay, 0, srv, responder, initiator)

	assert.NoError(t, srv.Close())
	assert.NoError(t, errWithTimeout(srvErrCh))
}

func testServerConcurrentTransportEstablishment(t *testing.T) {
	t.Parallel()

	dc := disc.NewMock()
	srv, srvErrCh, err := createServer(dc)
	require.NoError(t, err)

	// this way we can control the tests' difficulty
	initiatorsCount := 50
	respondersCount := 50
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	// store the number of transports each responder should handle
	listenersConnsCount := make(map[int]int)
	// mapping initiators to responders; one initiator performs a single connection,
	// while responders may handle from 0 to `initiatorsCount` connections
	pickedListeners := make([]int, 0, initiatorsCount)
	for i := 0; i < initiatorsCount; i++ {
		// pick random responder, which the initiator will connect to
		listenerIndex := rand.Intn(respondersCount)
		// increment the number of connections picked responder will handle
		listenersConnsCount[listenerIndex]++
		// map initiator to picked responder
		pickedListeners = append(pickedListeners, listenerIndex)
	}
	initiators := make([]*Client, 0, initiatorsCount)
	responders := make([]*Client, 0, respondersCount)
	listeners := make([]net.Listener, 0, respondersCount)
	for i := 0; i < initiatorsCount; i++ {
		initiators = append(initiators, createClient(t, dc, fmt.Sprintf("initiator_%d", i)))
	}
	for i := 0; i < respondersCount; i++ {
		pk, sk := cipher.GenerateKeyPair()

		c := NewClient(pk, sk, dc, SetLogger(logging.MustGetLogger(fmt.Sprintf("responder_%d", i))))
		if _, ok := listenersConnsCount[i]; ok {
			err := c.InitiateServerConnections(context.Background(), 1)
			require.NoError(t, err)
		}
		lis, err := c.Listen(port + uint16(i))
		require.NoError(t, err)

		responders = append(responders, c)
		listeners = append(listeners, lis)
	}
	totalListenerTpsCount := 0
	for _, connectionsCount := range listenersConnsCount {
		totalListenerTpsCount += connectionsCount
	}
	// channel to listen for `Accept` errors. Any single error must
	// fail the test
	acceptErrs := make(chan error, totalListenerTpsCount)
	var listenersTpsMX sync.Mutex
	listenersConns := make(map[int][]net.Conn, len(listenersConnsCount))
	var listenersWG sync.WaitGroup
	listenersWG.Add(totalListenerTpsCount)
	for i := range listeners {
		// only run `Accept` in case the responder was picked before
		if _, ok := listenersConnsCount[i]; !ok {
			continue
		}

		for connect := 0; connect < listenersConnsCount[i]; connect++ {
			// run responder
			go func(listenerIndex int) {
				defer listenersWG.Done()

				ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
				defer cancel()

				type result struct {
					Conn net.Conn
					Err  error
				}
				resultCh := make(chan result)

				go func() {
					conn, err := listeners[listenerIndex].Accept()
					resultCh <- result{conn, err}
				}()

				var conn net.Conn
				var err error

				select {
				case result := <-resultCh:
					conn, err = result.Conn, result.Err
				case <-ctx.Done():
					conn, err = nil, nil
				}

				if err != nil {
					acceptErrs <- err
					return
				}

				if conn != nil {
					// store connection
					listenersTpsMX.Lock()
					listenersConns[listenerIndex] = append(listenersConns[listenerIndex], conn)
					listenersTpsMX.Unlock()
				}
			}(i)
		}
	}

	// channel to listen for `Dial` errors. Any single error must
	// fail the test
	dialErrs := make(chan error, initiatorsCount)
	var initiatorsTpsMx sync.Mutex
	initiatorsTps := make([]net.Conn, 0, initiatorsCount)
	var initiatorsWG sync.WaitGroup
	initiatorsWG.Add(initiatorsCount)
	for i := range initiators {
		// run initiator
		go func(initiatorIndex int) {
			defer initiatorsWG.Done()

			responder := listeners[pickedListeners[initiatorIndex]].(*Listener)
			conn, err := initiators[initiatorIndex].Dial(context.Background(), responder.pk, responder.port)
			if err != nil {
				dialErrs <- err
			}

			// store conn
			initiatorsTpsMx.Lock()
			initiatorsTps = append(initiatorsTps, conn)
			initiatorsTpsMx.Unlock()
		}(i)
	}
	// wait for initiators
	initiatorsWG.Wait()
	close(dialErrs)
	err = <-dialErrs
	// single error should fail test
	require.NoError(t, err)
	// wait for responders
	listenersWG.Wait()
	close(acceptErrs)
	err = <-acceptErrs
	// single error should fail test
	require.NoError(t, err)
	checkConnCount(t, noDelay, len(listenersConnsCount)+initiatorsCount, srv)
	for i, initiator := range initiators {
		// get and check initiator's ServerConn
		initiatorSrvConn, ok := srv.getConn(initiator.pk)
		require.True(t, ok)
		require.Equal(t, initiator.pk, initiatorSrvConn.PK())

		// get and check initiator's ClientConn
		initiatorClientConn, ok := initiator.getConn(srv.pk)
		require.True(t, ok)
		require.Equal(t, srv.pk, initiatorClientConn.RemotePK())

		responder := responders[pickedListeners[i]]

		// get and check responder's ServerConn
		responderSrvConn, ok := srv.getConn(responder.pk)
		require.True(t, ok)
		require.Equal(t, responder.pk, responderSrvConn.PK())

		// get and check responder's ClientConn
		responderClientConn, ok := responder.getConn(srv.pk)
		require.True(t, ok)
		require.Equal(t, srv.pk, responderClientConn.RemotePK())

		// get initiator's nextConn
		nextInitID := getNextInitID(initiatorClientConn)
		initiatorNextConn, ok := initiatorSrvConn.getNext(nextInitID - 2)
		require.True(t, ok)
		require.NotNil(t, initiatorNextConn)
	}
	// close connections for responders
	for _, tps := range listenersConns {
		for _, tp := range tps {
			err := tp.Close()
			require.NoError(t, err)
		}
	}
	// close connections for initiators
	for _, tp := range initiatorsTps {
		err := tp.Close()
		require.NoError(t, err)
	}
	// close responders
	for _, responder := range responders {
		err := responder.Close()
		require.NoError(t, err)
	}
	// close initiators
	for _, initiator := range initiators {
		err := initiator.Close()
		require.NoError(t, err)
	}
	checkConnCount(t, smallDelay, 0, srv)
	for _, responder := range responders {
		checkConnCount(t, smallDelay, 0, responder)
	}
	for _, initiator := range initiators {
		checkConnCount(t, smallDelay, 0, initiator)
	}

	assert.NoError(t, srv.Close())
	assert.NoError(t, errWithTimeout(srvErrCh))
}

func testServerMessageConsistency(t *testing.T) {
	t.Parallel()

	dc := disc.NewMock()
	srv, srvErrCh, err := createServer(dc)
	require.NoError(t, err)

	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)
	initConn, respConn := dial(t, initiator, responder, port, noDelay)
	for i := 0; i < msgCount; i++ {
		// write message of 12 bytes
		_, err := initConn.Write([]byte(message))
		require.NoError(t, err)

		// create a receiving buffer of 5 bytes
		recBuff := make([]byte, bufSize)

		// read 5 bytes, 7 left
		n, err := respConn.Read(recBuff)
		require.NoError(t, err)
		require.Equal(t, n, len(recBuff))

		received := string(recBuff[:n])

		// read 5 more, 2 left
		n, err = respConn.Read(recBuff)
		require.NoError(t, err)
		require.Equal(t, n, len(recBuff))

		received += string(recBuff[:n])

		// read 2 bytes left
		n, err = respConn.Read(recBuff)
		require.NoError(t, err)
		require.Equal(t, n, len(message)-len(recBuff)*2)

		received += string(recBuff[:n])

		// received string must be equal to the sent one
		require.Equal(t, received, message)
	}
	require.NoError(t, closeClosers(initConn, respConn, responder, initiator))

	assert.NoError(t, srv.Close())
	assert.NoError(t, errWithTimeout(srvErrCh))
}

// Create a server, initiator, responder and connection between them then check if clients are connected to the server.
// Stop the server, then check if no client is connected and if connection is closed.
// Start the server again, check if clients reconnected and if `Dial()` and `Accept()` still work.
// Second argument indicates if the server restarts not on the same address, but on the random one.
func testServerReconnection(t *testing.T, randomAddr bool) {
	t.Parallel()

	dc := disc.NewMock()
	srv, srvErrCh, err := createServer(dc)
	require.NoError(t, err)

	serverAddr := srv.Addr()

	checkConnCount(t, noDelay, 0, srv)

	responder := createClient(t, dc, responderName)
	checkConnCount(t, smallDelay, 1, srv)

	initiator := createClient(t, dc, initiatorName)
	checkConnCount(t, smallDelay, 2, srv)

	initConn, respConn := dial(t, initiator, responder, port, noDelay)

	assert.NoError(t, srv.Close())
	assert.NoError(t, errWithTimeout(srvErrCh))

	checkTransportsClosed(t, initConn, respConn)
	checkConnCount(t, smallDelay, 0, srv)

	addr := ""
	if !randomAddr {
		addr = serverAddr
	}

	l, err := net.Listen("tcp", serverAddr)
	require.NoError(t, err)

	srv, err = NewServer(srv.pk, srv.sk, addr, l, dc)
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve()
		close(errCh)
	}()

	responder.pm.RemoveListener(port)
	checkConnCount(t, clientReconnectInterval+smallDelay, 2, srv)
	_, _ = dial(t, initiator, responder, port, smallDelay)

	assert.NoError(t, srv.Close())
	assert.NoError(t, errWithTimeout(errCh))
}

func createClient(t *testing.T, dc disc.APIClient, name string) *Client {
	pk, sk := cipher.GenerateKeyPair()

	client := NewClient(pk, sk, dc, SetLogger(logging.MustGetLogger(name)))
	require.NoError(t, client.InitiateServerConnections(context.Background(), 1))

	return client
}

func createServer(dc disc.APIClient) (srv *Server, srvErr <-chan error, err error) {
	pk, sk := cipher.GenerateKeyPair()

	l, err := nettest.NewLocalListener("tcp")
	if err != nil {
		return nil, nil, err
	}

	srv, err = NewServer(pk, sk, "", l, dc)
	if err != nil {
		return nil, nil, err
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve()
		close(errCh)
	}()

	return srv, errCh, nil
}

func dial(t *testing.T, initiator, responder *Client, port uint16, delay time.Duration) (initTp, respTp net.Conn) {
	require.NoError(t, testWithTimeout(delay, func() error {
		ctx := context.TODO()

		listener, err := responder.Listen(port)
		if err != nil {
			return err
		}

		initTp, err = initiator.Dial(ctx, responder.pk, port)
		if err != nil {
			return err
		}

		respTp, err = listener.Accept()
		return err
	}))
	return initTp, respTp
}
