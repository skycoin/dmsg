package dmsg

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/disc"
)

func BenchmarkTransportMap_Read(b *testing.B) {
	initTr, respTr, err := createClientsMap()
	if err != nil {
		b.Fatal(err)
	}

	const messageSize = 50000
	const bufSize = 10
	message := bytes.Repeat([]byte("a"), messageSize)

	go func() {
		for {
			initTr.Write(message) // nolint:errcheck
			time.Sleep(10 * time.Microsecond)
		}
	}()

	b.ResetTimer()
	buf := make([]byte, bufSize)
	for i := 0; i < b.N; i++ {
		_, err := respTr.Read(buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransportMap_Write(b *testing.B) {
	initTr, _, err := createClientsMap()
	if err != nil {
		b.Fatal(err)
	}

	const bufSize = 50000
	buf := make([]byte, bufSize)
	go func() {
		for {
			initTr.Read(buf) // nolint:errcheck
			time.Sleep(10 * time.Microsecond)
		}
	}()

	b.ResetTimer()
	message := []byte("a")
	for i := 0; i < b.N; i++ {
		n, err := initTr.Write(message)
		if err != nil {
			b.Fatal(err)
		}
		if n != len(message) {
			b.Fatal("not enough bytes written")
		}
	}
}

func createClientsMap() (initTp, respTp *TransportMap, err error) {
	dc := disc.NewMock()
	ctx := context.TODO()

	if err := createServer(dc); err != nil {
		return nil, nil, err
	}

	responderPK, responderSK := cipher.GenerateKeyPair()
	initiatorPK, initiatorSK := cipher.GenerateKeyPair()
	responder := NewClientMap(responderPK, responderSK, dc, SetLoggerMap(logging.MustGetLogger("responder")))
	err = responder.InitiateServerConnections(ctx, 1)
	if err != nil {
		return nil, nil, err
	}

	initiator := NewClientMap(initiatorPK, initiatorSK, dc, SetLoggerMap(logging.MustGetLogger("initiator")))
	err = initiator.InitiateServerConnections(ctx, 1)
	if err != nil {
		return nil, nil, err
	}

	initTp, err = initiator.Dial(ctx, responder.pk)
	if err != nil {
		return nil, nil, err
	}

	respTp, err = responder.Accept(ctx)
	if err != nil {
		return nil, nil, err
	}

	return initTp, respTp, nil
}

func createServer(dc disc.APIClient) error {
	serverPK, serverSK := cipher.GenerateKeyPair()

	l, err := net.Listen("tcp", "")
	if err != nil {
		return err
	}

	s, err := NewServer(serverPK, serverSK, "", l, dc)
	if err != nil {
		return err
	}

	go s.Serve() //nolint:errcheck
	return nil
}
