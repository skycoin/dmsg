package dmsg

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/skycoin/skycoin/src/util/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/disc"
	"github.com/skycoin/dmsg/noise"
)

const (
	responderName = "responder"
	initiatorName = "initiator"
	message       = "Hello there!"
	msgCount      = 100
	bufSize       = 5
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

	var fullNextConns [math.MaxUint16 + 1]*NextConn
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
				nextConns: [math.MaxUint16 + 1]*NextConn{
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
	}

	var wg sync.WaitGroup
	wg.Add(connsCount)
	for i := 0; i < connsCount; i++ {
		go func() {
			_, err := serverConn.addNext(context.Background(), &NextConn{})
			require.NoError(t, err)

			wg.Done()
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
	sPK, sSK := cipher.GenerateKeyPair()
	dc := disc.NewMock()

	l, err := net.Listen("tcp", "")
	require.NoError(t, err)

	// When calling 'NewServer', if the provided net.Listener is already a noise.Listener,
	// An error should be returned.
	t.Run("Already wrapped listener fails", func(t *testing.T) {
		wrappedL := noise.WrapListener(l, sPK, sSK, false, noise.HandshakeXK)
		s, err := NewServer(sPK, sSK, "", wrappedL, dc)
		assert.Equal(t, ErrListenerAlreadyWrappedToNoise, err)
		assert.Nil(t, s)
	})

	srv, err := NewServer(sPK, sSK, "", l, dc)
	require.NoError(t, err)

	go srv.Serve() //nolint:errcheck

	time.Sleep(smallDelay)

	assert.NoError(t, srv.Close())
}

// TestServer_Serve ensures that Server processes request frames and
// instantiates transports properly.
func TestServer_Serve(t *testing.T) {
	dc := disc.NewMock()

	srv := createServer(t, dc, "")
	go srv.Serve() //nolint:errcheck

	// connect two clients, establish transport, check if there are
	// two ServerConn's and that both conn's `nextConn` is filled correctly
	t.Run("Transport establishes", func(t *testing.T) {
		testTransportEstablishment(t, dc, srv)
	})

	t.Run("Transport establishes concurrently", func(t *testing.T) {
		testConcurrentTransportEstablishment(t, dc, srv)
	})

	t.Run("Failed accepts do not result in hang", func(t *testing.T) {
		t.Parallel()
		testFailedAccepts(t, dc)
	})

	t.Run("Sent/received message is consistent", func(t *testing.T) {
		t.Parallel()
		testMessageConsistency(t, dc)
	})

	t.Run("Capped transport buffer does not result in hang", func(t *testing.T) {
		t.Parallel()
		testCappedTransport(t, dc)
	})

	t.Run("Self dialing works", func(t *testing.T) {
		t.Parallel()
		testSelfDialing(t, dc)
	})

	t.Run("Server disconnection closes transports", func(t *testing.T) {
		t.Parallel()
		testServerDisconnection(t)
	})

	t.Run("Reconnection to server succeeds", func(t *testing.T) {
		t.Run("Same address", func(t *testing.T) {
			t.Parallel()
			testServerReconnection(t, false)
		})

		t.Run("Random address", func(t *testing.T) {
			t.Parallel()
			testServerReconnection(t, true)
		})
	})
}

func testServerDisconnection(t *testing.T) {
	dc := disc.NewMock()
	srv := createServer(t, dc, "")
	var srvStartErr error
	srvDone := make(chan struct{})
	go func() {
		if err := srv.Serve(); err != nil {
			srvStartErr = err
		}

		close(srvDone)
	}()
	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)
	initiatorTransport, responderTransport := dial(t, initiator, responder, noDelay)
	testTransportMessaging(t, initiatorTransport, responderTransport)
	err := srv.Close()
	require.NoError(t, err)
	<-srvDone
	// TODO: remove log, uncomment when bug is fixed
	log.Printf("SERVE ERR: %v", srvStartErr)
	// require.NoError(t, sStartErr)
	/*time.Sleep(largeDelay)

	tp, ok := bTransport.(*Transport)
	require.Equal(t, true, ok)
	require.Equal(t, true, tp.IsClosed())

	tp, ok = aTransport.(*Transport)
	require.Equal(t, true, ok)
	require.Equal(t, true, tp.IsClosed())*/
}

func testSelfDialing(t *testing.T, dc disc.APIClient) {
	client := createClient(t, dc, "client")
	selfWrTp, selfRdTp := dial(t, client, client, noDelay)
	// try to write/read message to/from self
	testTransportMessaging(t, selfWrTp, selfRdTp)
	require.NoError(t, closeClosers(selfRdTp, selfWrTp, client))
}

func testTransportMessaging(t *testing.T, init *Transport, resp *Transport) {
	for i := 0; i < msgCount; i++ {
		_, err := init.Write([]byte(message))
		require.NoError(t, err)

		recvBuf := make([]byte, bufSize)
		for i := 0; i < len(message); i += bufSize {
			_, err := resp.Read(recvBuf)
			require.NoError(t, err)
		}
	}
}

func testCappedTransport(t *testing.T, dc disc.APIClient) {
	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)
	responderWrTransport, err := responder.Dial(context.Background(), initiator.pk)
	require.NoError(t, err)
	_, err = initiator.Accept(context.Background())
	require.NoError(t, err)
	msg := []byte(message)
	// exact iterations to fill the receiving buffer and hang `Write`
	iterationsToDo := tpBufCap/len(msg) + 1
	// fill the buffer, but no block yet
	for i := 0; i < iterationsToDo-1; i++ {
		_, err = responderWrTransport.Write(msg)
		require.NoError(t, err)
	}
	// block on `Write`
	var blockedWriteErr error
	blockedWriteDone := make(chan struct{})
	go func() {
		_, blockedWriteErr = responderWrTransport.Write(msg)
		close(blockedWriteDone)
	}()
	// wait till it's definitely blocked
	initiatorWrTransport, responderRdTransport := dial(t, initiator, responder, smallDelay)
	// try to write/read message via the new transports
	for i := 0; i < msgCount; i++ {
		_, err := initiatorWrTransport.Write(msg)
		require.NoError(t, err)

		recBuff := make([]byte, len(msg))
		_, err = responderRdTransport.Read(recBuff)
		require.NoError(t, err)

		require.Equal(t, recBuff, msg)
	}
	err = responderWrTransport.Close()
	require.NoError(t, err)
	<-blockedWriteDone
	require.Error(t, blockedWriteErr)
	require.NoError(t, closeClosers(initiatorWrTransport, responderRdTransport, responder, initiator))
}

func testFailedAccepts(t *testing.T, dc disc.APIClient) {
	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)
	initiatorTransport, responderTransport := dial(t, initiator, responder, noDelay)
	readWriteStop := make(chan struct{})
	readWriteDone := make(chan struct{})
	var readErr, writeErr error
	go func() {
		// read/write to/from transport until the stop signal arrives
		for {
			select {
			case <-readWriteStop:
				close(readWriteDone)
				return
			default:
				if _, writeErr = initiatorTransport.Write([]byte(message)); writeErr != nil {
					close(readWriteDone)
					return
				}
				if _, readErr = responderTransport.Read([]byte(message)); readErr != nil {
					close(readWriteDone)
					return
				}
			}
		}
	}()
	var err error
	// continue creating transports until the error occurs
	for {
		ctx := context.Background()
		if _, err = responder.Dial(ctx, initiator.pk); err != nil {
			break
		}
	}
	// must be error
	require.Error(t, err)
	// the same as above, transport is created by another client
	for {
		ctx := context.Background()
		if _, err = initiator.Dial(ctx, responder.pk); err != nil {
			break
		}
	}
	// must be error
	require.Error(t, err)
	// wait more time to ensure that the initially created transport works
	time.Sleep(smallDelay)
	require.NoError(t, closeClosers(responderTransport, initiatorTransport))
	// stop reading/writing goroutines
	close(readWriteStop)
	<-readWriteDone
	// check that the initial transport had been working properly all the time
	// if any error, it must be `io.EOF` for reader
	if readErr != io.EOF {
		require.NoError(t, readErr)
	}
	// if any error, it must be `io.ErrClosedPipe` for writer
	if writeErr != io.ErrClosedPipe {
		require.NoError(t, writeErr)
	}
	require.NoError(t, closeClosers(responder, initiator))
}

func testTransportEstablishment(t *testing.T, dc disc.APIClient, srv *Server) {
	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)
	initiatorTransport, responderTransport := dial(t, initiator, responder, noDelay)
	// must be 2 ServerConn's
	checkConnCount(t, srv, 2, noDelay)
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
	require.NoError(t, closeClosers(responderTransport, initiatorTransport, responder, initiator))
	checkConnCount(t, srv, 0, smallDelay)
	checkConnCount(t, responder, 0, smallDelay)
	checkConnCount(t, initiator, 0, smallDelay)
}

func testConcurrentTransportEstablishment(t *testing.T, dc disc.APIClient, srv *Server) {
	// this way we can control the tests' difficulty
	initiatorsCount := 50
	respondersCount := 50
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	// store the number of transports each responder should handle
	respondersTpsCount := make(map[int]int)
	// mapping initiators to responders; one initiator performs a single connection,
	// while responders may handle from 0 to `initiatorsCount` connections
	pickedResponders := make([]int, 0, initiatorsCount)
	for i := 0; i < initiatorsCount; i++ {
		// pick random responder, which the initiator will connect to
		responder := rand.Intn(respondersCount)
		// increment the number of connections picked responder will handle
		respondersTpsCount[responder]++
		// map initiator to picked responder
		pickedResponders = append(pickedResponders, responder)
	}
	initiators := make([]*Client, 0, initiatorsCount)
	responders := make([]*Client, 0, respondersCount)
	for i := 0; i < initiatorsCount; i++ {
		initiators = append(initiators, createClient(t, dc, fmt.Sprintf("initiator_%d", i)))
	}
	for i := 0; i < respondersCount; i++ {
		pk, sk := cipher.GenerateKeyPair()

		c := NewClient(pk, sk, dc, SetLogger(logging.MustGetLogger(fmt.Sprintf("remote_%d", i))))
		if _, ok := respondersTpsCount[i]; ok {
			err := c.InitiateServerConnections(context.Background(), 1)
			require.NoError(t, err)
		}
		responders = append(responders, c)
	}
	totalResponderTpsCount := 0
	for _, connectionsCount := range respondersTpsCount {
		totalResponderTpsCount += connectionsCount
	}
	// channel to listen for `Accept` errors. Any single error must
	// fail the test
	acceptErrs := make(chan error, totalResponderTpsCount)
	var respondersTpsMX sync.Mutex
	respondersTps := make(map[int][]*Transport, len(respondersTpsCount))
	var respondersWG sync.WaitGroup
	respondersWG.Add(totalResponderTpsCount)
	for i := range responders {
		// only run `Accept` in case the responder was picked before
		if _, ok := respondersTpsCount[i]; !ok {
			continue
		}
		for connect := 0; connect < respondersTpsCount[i]; connect++ {
			// run responder
			go func(responderIndex int) {
				var (
					transport *Transport
					err       error
				)

				transport, err = responders[responderIndex].Accept(context.Background())
				if err != nil {
					acceptErrs <- err
				}

				// store transport
				respondersTpsMX.Lock()
				respondersTps[responderIndex] = append(respondersTps[responderIndex], transport)
				respondersTpsMX.Unlock()

				respondersWG.Done()
			}(i)
		}
	}
	// channel to listen for `Dial` errors. Any single error must
	// fail the test
	dialErrs := make(chan error, initiatorsCount)
	var initiatorsTpsMx sync.Mutex
	initiatorsTps := make([]*Transport, 0, initiatorsCount)
	var initiatorsWG sync.WaitGroup
	initiatorsWG.Add(initiatorsCount)
	for i := range initiators {
		// run initiator
		go func(initiatorIndex int) {
			var (
				transport *Transport
				err       error
			)

			responder := responders[pickedResponders[initiatorIndex]]
			transport, err = initiators[initiatorIndex].Dial(context.Background(), responder.pk)
			if err != nil {
				dialErrs <- err
			}

			// store transport
			initiatorsTpsMx.Lock()
			initiatorsTps = append(initiatorsTps, transport)
			initiatorsTpsMx.Unlock()

			initiatorsWG.Done()
		}(i)
	}
	// wait for initiators
	initiatorsWG.Wait()
	close(dialErrs)
	err := <-dialErrs
	// single error should fail test
	require.NoError(t, err)
	// wait for responders
	respondersWG.Wait()
	close(acceptErrs)
	err = <-acceptErrs
	// single error should fail test
	require.NoError(t, err)
	checkConnCount(t, srv, len(respondersTpsCount)+initiatorsCount, noDelay)
	for i, initiator := range initiators {
		// get and check initiator's ServerConn
		initiatorSrvConn, ok := srv.getConn(initiator.pk)
		require.True(t, ok)
		require.Equal(t, initiator.pk, initiatorSrvConn.PK())

		// get and check initiator's ClientConn
		initiatorClientConn, ok := initiator.getConn(srv.pk)
		require.True(t, ok)
		require.Equal(t, srv.pk, initiatorClientConn.RemotePK())

		responder := responders[pickedResponders[i]]

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
	// close transports for responders
	for _, tps := range respondersTps {
		for _, tp := range tps {
			err := tp.Close()
			require.NoError(t, err)
		}
	}
	// close transports for initiators
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
	checkConnCount(t, srv, 0, smallDelay)
	for _, responder := range responders {
		checkConnCount(t, responder, 0, smallDelay)
	}
	for _, initiator := range initiators {
		checkConnCount(t, initiator, 0, smallDelay)
	}
}

func testMessageConsistency(t *testing.T, dc disc.APIClient) {
	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)
	initiatorTransport, responderTransport := dial(t, initiator, responder, noDelay)
	for i := 0; i < msgCount; i++ {
		// write message of 12 bytes
		_, err := initiatorTransport.Write([]byte(message))
		require.NoError(t, err)

		// create a receiving buffer of 5 bytes
		recBuff := make([]byte, 5)

		// read 5 bytes, 7 left
		n, err := responderTransport.Read(recBuff)
		require.NoError(t, err)
		require.Equal(t, n, len(recBuff))

		received := string(recBuff[:n])

		// read 5 more, 2 left
		n, err = responderTransport.Read(recBuff)
		require.NoError(t, err)
		require.Equal(t, n, len(recBuff))

		received += string(recBuff[:n])

		// read 2 bytes left
		n, err = responderTransport.Read(recBuff)
		require.NoError(t, err)
		require.Equal(t, n, len(message)-len(recBuff)*2)

		received += string(recBuff[:n])

		// received string must be equal to the sent one
		require.Equal(t, received, message)
	}
	require.NoError(t, closeClosers(initiatorTransport, responderTransport, responder, initiator))
}

// Create a server, initiator, responder and transport between them then check if clients are connected to the server.
// Stop the server, then check if no client is connected and if transport is closed.
// Start the server again, check if clients reconnected and if `Dial()` and `Accept()` still work.
// Second argument indicates if the server restarts not on the same address, but on the random one.
func testServerReconnection(t *testing.T, randomAddr bool) {
	dc := disc.NewMock()
	srv := createServer(t, dc, "")

	serverAddr := srv.Addr()

	go srv.Serve() // nolint:errcheck

	checkConnCount(t, srv, 0, noDelay)

	responder := createClient(t, dc, responderName)
	checkConnCount(t, srv, 1, smallDelay)

	initiator := createClient(t, dc, initiatorName)
	checkConnCount(t, srv, 2, smallDelay)

	initiatorTransport, responderTransport := dial(t, initiator, responder, noDelay)

	assert.NoError(t, srv.Close())
	checkTransportsClosed(t, initiatorTransport, responderTransport)
	checkConnCount(t, srv, 0, smallDelay)

	addr := ""
	if !randomAddr {
		addr = serverAddr
	}

	l, err := net.Listen("tcp", serverAddr)
	require.NoError(t, err)

	srv, err = NewServer(srv.pk, srv.sk, addr, l, dc)
	require.NoError(t, err)

	go srv.Serve() // nolint:errcheck

	checkConnCount(t, srv, 2, clientReconnectInterval+smallDelay)
	_, _ = dial(t, initiator, responder, smallDelay)

	assert.NoError(t, srv.Close())
}

func createClient(t *testing.T, dc disc.APIClient, name string) *Client {
	pk, sk := cipher.GenerateKeyPair()

	client := NewClient(pk, sk, dc, SetLogger(logging.MustGetLogger(name)))
	require.NoError(t, client.InitiateServerConnections(context.Background(), 1))

	return client
}

func createServer(t *testing.T, dc disc.APIClient, addr string) *Server {
	pk, sk := cipher.GenerateKeyPair()

	l, err := nettest.NewLocalListener("tcp")
	require.NoError(t, err)

	srv, err := NewServer(pk, sk, addr, l, dc)
	require.NoError(t, err)

	return srv
}

// TODO: update comments mentioning a & b
// Given two client instances (a & b) and a server instance (s),
// Client b should be able to dial a transport with client b
// Data should be sent and delivered successfully via the transport.
// TODO: fix this.
func TestNewClient(t *testing.T) {
	sPK, sSK := cipher.GenerateKeyPair()
	sAddr := "127.0.0.1:8081"

	const tpCount = 10

	dc := disc.NewMock()

	l, err := net.Listen("tcp", sAddr)
	require.NoError(t, err)

	srv, err := NewServer(sPK, sSK, "", l, dc)
	require.NoError(t, err)

	log.Println(srv.Addr())

	go srv.Serve() //nolint:errcheck

	responder := createClient(t, dc, responderName)
	initiator := createClient(t, dc, initiatorName)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < tpCount; i++ {
			responderDone := make(chan struct{})
			var responderTp *Transport
			go func() {
				var err error
				responderTp, err = responder.Accept(context.Background())
				catch(err)
				close(responderDone)
			}()

			initiatorTp, err := initiator.Dial(context.Background(), responder.pk)
			catch(err)

			<-responderDone
			catch(err)

			for j := 0; j < msgCount; j++ {
				pay := []byte(fmt.Sprintf("This is message %d!", j))
				_, err := responderTp.Write(pay)
				catch(err)
				_, err = initiatorTp.Read(pay)
				catch(err)
			}

			catch(closeClosers(responderTp, initiatorTp))
		}
	}()

	for i := 0; i < tpCount; i++ {
		initiatorDone := make(chan struct{})
		var initiatorErr error
		var initiatorTp *Transport
		go func() {
			initiatorTp, initiatorErr = initiator.Accept(context.Background())
			close(initiatorDone)
		}()

		responderTp, err := responder.Dial(context.Background(), initiator.pk)
		require.NoError(t, err)

		<-initiatorDone
		require.NoError(t, initiatorErr)

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

	assert.NoError(t, srv.Close())
}

func dial(t *testing.T, initiator, responder *Client, delay time.Duration) (initTp *Transport, respTp *Transport) {
	var err error

	require.NoError(t, testWithTimeout(delay, func() error {
		ctx := context.TODO()

		initTp, err = initiator.Dial(ctx, responder.pk)
		if err != nil {
			return err
		}

		respTp, err = responder.Accept(ctx)
		return err
	}))
	return initTp, respTp
}
