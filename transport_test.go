package dmsg

import (
	"bytes"
	"context"
	"testing"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/stretchr/testify/assert"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
)

func TestNewTransport(t *testing.T) {
	log := logging.MustGetLogger("dmsg_test")
	tr := NewTransport(nil, log, Addr{}, Addr{}, 0, func(id uint16) {})
	assert.NotNil(t, tr)
}

func BenchmarkNewTransport(b *testing.B) {
	log := logging.MustGetLogger("dmsg_test")
	for i := 0; i < b.N; i++ {
		NewTransport(nil, log, Addr{}, Addr{}, 0, func(id uint16) {})
	}
}

func TestTransport_close(t *testing.T) {
	log := logging.MustGetLogger("dmsg_test")
	tr := NewTransport(nil, log, Addr{}, Addr{}, 0, func(id uint16) {})

	closed := tr.close()

	t.Run("Valid close() result (1st attempt)", func(t *testing.T) {
		assert.True(t, closed)
	})

	t.Run("Channel closed (1st attempt)", func(t *testing.T) {
		_, ok := <-tr.done
		assert.False(t, ok)
	})

	closed = tr.close()

	t.Run("Valid close() result (2nd attempt)", func(t *testing.T) {
		assert.False(t, closed)
	})

	t.Run("Channel closed (2nd attempt)", func(t *testing.T) {
		_, ok := <-tr.done
		assert.False(t, ok)
	})

	t.Run("No panic with nil pointer receiver", func(t *testing.T) {
		var tr1, tr2 *Transport
		assert.NoError(t, tr1.Close())
		assert.False(t, tr2.close())
	})
}

func BenchmarkTransport_Read(b *testing.B) {
	initTr, respTr, err := createBenchmarkClients()
	if err != nil {
		b.Error(err)
	}

	const messageSize = 50000
	const bufSize = 10
	message := bytes.Repeat([]byte("a"), messageSize)
	go func() {
		for {
			if _, err := initTr.Write(message); err != nil {
				b.Error(err)
			}
		}
	}()

	b.ResetTimer()
	buf := make([]byte, bufSize)
	for i := 0; i < b.N; i++ {
		if _, err := respTr.Read(buf); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkTransport_Write(b *testing.B) {
	initTr, _, err := createBenchmarkClients()
	if err != nil {
		b.Error(err)
	}

	const bufSize = 50000
	buf := make([]byte, bufSize)
	go func() {
		for {
			if _, err := initTr.Read(buf); err != nil {
				b.Error(err)
			}
		}
	}()

	b.ResetTimer()
	message := []byte("a")
	for i := 0; i < b.N; i++ {
		if _, err := initTr.Write(message); err != nil {
			b.Error(err)
		}
	}
}

func createBenchmarkClients() (initTp, respTp *Transport, err error) {
	dc := disc.NewMock()
	ctx := context.TODO()

	if _, _, err := createServer(dc); err != nil {
		return nil, nil, err
	}

	responderPK, responderSK := cipher.GenerateKeyPair()
	initiatorPK, initiatorSK := cipher.GenerateKeyPair()
	responder := NewClient(responderPK, responderSK, dc, SetLogger(logging.MustGetLogger("responder")))
	err = responder.InitiateServerConnections(ctx, 1)
	if err != nil {
		return nil, nil, err
	}

	initiator := NewClient(initiatorPK, initiatorSK, dc, SetLogger(logging.MustGetLogger("initiator")))
	err = initiator.InitiateServerConnections(ctx, 1)
	if err != nil {
		return nil, nil, err
	}

	initTp, err = initiator.Dial(ctx, responder.pk, port)
	if err != nil {
		return nil, nil, err
	}

	listener, err := responder.Listen(port)
	if err != nil {
		return nil, nil, err
	}

	respTp, err = listener.AcceptTransport()
	if err != nil {
		return nil, nil, err
	}

	return initTp, respTp, nil
}

// TODO(evanlinjin): Re-enable this test when it passes.
//func TestTransport(t *testing.T) {
//	mp := func() (c1, c2 net.Conn, stop func(), err error) {
//		respPK, respSK := cipher.GenerateKeyPair()
//		initPK, initSK := cipher.GenerateKeyPair()
//
//		var initPort, respPort uint16 = 1563, 1563
//
//		dc := disc.NewMock()
//
//		srv, err := CreateDmsgTestServer(dc)
//		if err != nil {
//			return nil, nil, nil, err
//		}
//		defer func() { require.NoError(t, srv.Close()) }()
//
//		respC := NewClient(respPK, respSK, dc)
//		initC := NewClient(initPK, initSK, dc)
//
//		if err := respC.InitiateServerConnections(context.Background(), 1); err != nil {
//			return nil, nil, nil, err
//		}
//
//		if err := initC.InitiateServerConnections(context.Background(), 1); err != nil {
//			return nil, nil, nil, err
//		}
//
//		initL, err := initC.Listen(initPort)
//		if err != nil {
//			return nil, nil, nil, err
//		}
//
//		respL, err := respC.Listen(respPort)
//		if err != nil {
//			return nil, nil, nil, err
//		}
//
//		initTp, err := initC.Dial(context.Background(), respPK, respPort)
//		if err != nil {
//			return nil, nil, nil, err
//		}
//
//		respTp, err := respL.AcceptTransport()
//		if err != nil {
//			return nil, nil, nil, err
//		}
//
//		closeFunc := func() {
//			require.NoError(t, initTp.Close())
//			require.NoError(t, respTp.Close())
//			require.NoError(t, initL.Close())
//			require.NoError(t, respL.Close())
//			require.NoError(t, initC.Close())
//			require.NoError(t, respC.Close())
//		}
//
//		return initTp, respTp, closeFunc, nil
//	}
//
//	nettest.TestConn(t, mp)
//}
