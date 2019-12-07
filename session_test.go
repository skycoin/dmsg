package dmsg

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"

	"github.com/SkycoinProject/dmsg/cipher"
)

/*
	A -> S -> B
*/
//func TestSession(t *testing.T) {
//	sPK, sSK := GenKeyPair(t, "server")
//	aPK, aSK := GenKeyPair(t, "client A")
//	bPK, bSK := GenKeyPair(t, "client B")
//	porter := netutil.NewPorter(netutil.PorterMinEphemeral)
//	getSes, addSes := MakeGetter()
//
//	makeSessionPair := func(cPK cipher.PubKey, cSK cipher.SecKey, logSuffix string) (cSes, sSes *ClientSession) {
//		cConn, sConn := net.Pipe()
//
//		cErr := make(chan error, 1)
//		go func() {
//			ses, err := InitiateSession(logging.MustGetLogger("client_"+logSuffix), porter, cConn, cSK, cPK, sPK)
//			cSes = ses
//			cErr <- err
//			close(cErr)
//		}()
//
//		sSes, err := RespondSession(logging.MustGetLogger("server_"+logSuffix), getSes, sConn, sSK, sPK)
//		require.NoError(t, err)
//		addSes(sSes)
//		require.NoError(t, <-cErr)
//
//		return cSes, sSes
//	}
//
//	aSes, aSrv := makeSessionPair(aPK, aSK, "A")
//	bSes, bSrv := makeSessionPair(bPK, bSK, "B")
//
//	makePiper := func(src, adj, dst *Session) nettest.MakePipe {
//
//		port := uint16(1)
//		getPort := func() uint16 {
//			for {
//				if port++; port != 0 {
//					return port
//				}
//			}
//		}
//
//		return func() (c1, c2 net.Conn, stop func(), err error) {
//			// Make dst listen.
//			dstAddr := Addr{PK: dst.LocalPK(), Port: getPort()}
//			dstLis := AddListener(t, porter, dstAddr)
//
//			// Ensure we are accepting.
//			accepts := make(chan error, 2)
//			go func() { accepts <- adj.acceptAndProxyStream() }()
//			go func() { accepts <- dst.acceptClientStream() }()
//
//			// Make src dial to dst.
//			c1, err = src.dialClientStream(context.TODO(), dstAddr)
//			if err != nil {
//				return
//			}
//
//			// Make dst accept src.
//			c2, err = dstLis.AcceptStream()
//			if err != nil {
//				return
//			}
//
//			// Check accepts.
//			for i := 0; i < 2; i++ {
//				if err = <-accepts; err != nil {
//					return
//				}
//			}
//			close(accepts)
//
//			// Make stop.
//			stop = func() {
//				_ = c1.Close()
//				_ = c2.Close()
//				_ = dstLis.Close()
//			}
//
//			return c1, c2, stop, nil
//		}
//	}
//
//	DoTestConn(t, makePiper(aSes, aSrv, bSes))
//	DoTestConn(t, makePiper(bSes, bSrv, aSes))
//	nettest.TestConn(t, makePiper(aSes, aSrv, bSes))
//	nettest.TestConn(t, makePiper(bSes, bSrv, aSes))
//}

// This is a simplified test for a piped connection pair.
func DoTestConn(t *testing.T, makePipe nettest.MakePipe) {
	c1, c2, stop, err := makePipe()
	require.NoError(t, err)

	defer func() {
		stop()
		fmt.Println("stopped!")
	}()

	for i := 1; i < 2000; i++ {
		msg := cipher.RandByte(i)

		n1, err := c1.Write(msg)
		require.NoError(t, err)
		require.Equal(t, len(msg), n1)

		in := make([]byte, len(msg))
		n2, err := c2.Read(in)
		require.NoError(t, err)
		require.Equal(t, len(msg), n2)

		require.Equal(t, msg, in)
	}
}
