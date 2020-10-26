package dmsg

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/skycoin/dmsg/encodedecoder"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/yamux"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/noise"
)

// SessionCommon contains the common fields and methods used by a session, whether it be it from the client or server
// perspective.
type SessionCommon struct {
	entity *EntityCommon // back reference
	rPK    cipher.PubKey // remote pk

	netConn net.Conn // underlying net.Conn (TCP connection to the dmsg server)
	ys      *yamux.Session
	ns      *noise.Noise
	nMap    noise.NonceMap
	rMx     sync.Mutex
	wMx     sync.Mutex

	log logrus.FieldLogger

	encrypt bool

	ed encodedecoder.EncodeDecoder
}

// GetConn returns underlying TCP `net.Conn`.
func (sc *SessionCommon) GetConn() net.Conn {
	return sc.netConn
}

func (sc *SessionCommon) initClient(entity *EntityCommon, conn net.Conn, rPK cipher.PubKey) error {
	ns, err := noise.New(noise.HandshakeXK, noise.Config{
		LocalPK:   entity.pk,
		LocalSK:   entity.sk,
		RemotePK:  rPK,
		Initiator: true,
	})
	if err != nil {
		return err
	}

	r := bufio.NewReader(conn)
	if err := noise.InitiatorHandshake(ns, r, conn); err != nil {
		return err
	}
	if r.Buffered() > 0 {
		return ErrSessionHandshakeExtraBytes
	}

	ySes, err := yamux.Client(conn, yamux.DefaultConfig())
	if err != nil {
		return err
	}

	sc.entity = entity
	sc.rPK = rPK
	sc.netConn = conn
	sc.ys = ySes
	sc.ns = ns
	sc.nMap = make(noise.NonceMap)
	sc.log = entity.log.WithField("session", ns.RemoteStatic())
	return nil
}

func (sc *SessionCommon) initServer(entity *EntityCommon, conn net.Conn) error {
	ns, err := noise.New(noise.HandshakeXK, noise.Config{
		LocalPK:   entity.pk,
		LocalSK:   entity.sk,
		Initiator: false,
	})
	if err != nil {
		return err
	}

	r := bufio.NewReader(conn)
	if err := noise.ResponderHandshake(ns, r, conn); err != nil {
		return err
	}
	if r.Buffered() > 0 {
		return ErrSessionHandshakeExtraBytes
	}

	ySes, err := yamux.Server(conn, yamux.DefaultConfig())
	if err != nil {
		return err
	}

	sc.entity = entity
	sc.rPK = ns.RemoteStatic()
	sc.netConn = conn
	sc.ys = ySes
	sc.ns = ns
	sc.nMap = make(noise.NonceMap)
	sc.log = entity.log.WithField("session", ns.RemoteStatic())
	return nil
}

// writeEncryptedGob encrypts with noise and prefixed with uint16 (2 additional bytes).
func (sc *SessionCommon) writeObject(w io.Writer, obj SignedObject) error {
	p := []byte(obj)

	if sc.encrypt {
		sc.wMx.Lock()
		p = sc.ns.EncryptUnsafe(obj)
		sc.wMx.Unlock()

		p = append(make([]byte, 2), p...)
		binary.BigEndian.PutUint16(p, uint16(len(p)-2))
	} else {
		pLen := strconv.FormatUint(uint64(uint16(len(p)-2)), 10)
		pLenBytes := []byte(pLen)
		p = append(make([]byte, 5), p...)
		for i := 0; i < 5; i++ {
			if len(pLenBytes) > i {
				p[i] = pLenBytes[i]
			}
		}
	}

	fmt.Printf("ENCRYPTED OBJECT: %v\n", p)
	_, err := w.Write(p)
	return err
}

func (sc *SessionCommon) readObject(r io.Reader) (SignedObject, error) {
	var pb []byte
	if sc.encrypt {
		lb := make([]byte, 2)
		if _, err := io.ReadFull(r, lb); err != nil {
			return nil, err
		}
		pb = make([]byte, binary.BigEndian.Uint16(lb))
		if _, err := io.ReadFull(r, pb); err != nil {
			return nil, err
		}
	} else {
		lbBytes := make([]byte, 5)
		if _, err := io.ReadFull(r, lbBytes); err != nil {
			return nil, err
		}
		lastIdx := bytes.Index(lbBytes, []byte{0})
		if lastIdx == -1 {
			lastIdx = 5
		}

		lb, err := strconv.ParseUint(string(lbBytes[:lastIdx]), 10, 64)
		if err != nil {
			return nil, err
		}

		pb := make([]byte, lb)
		if _, err := io.ReadFull(r, pb); err != nil {
			return nil, err
		}
	}

	fmt.Printf("GOT pb: %s\n", string(pb))

	sc.rMx.Lock()
	defer sc.rMx.Unlock()

	if sc.nMap == nil {
		return nil, ErrSessionClosed
	}

	if !sc.encrypt {
		return pb, nil
	}

	return sc.ns.DecryptWithNonceMap(sc.nMap, pb)
}

func (sc *SessionCommon) localSK() cipher.SecKey { return sc.entity.sk }

// LocalPK returns the local public key of the session.
func (sc *SessionCommon) LocalPK() cipher.PubKey { return sc.entity.pk }

// RemotePK returns the remote public key of the session.
func (sc *SessionCommon) RemotePK() cipher.PubKey { return sc.rPK }

// LocalTCPAddr returns the local address of the underlying TCP connection.
func (sc *SessionCommon) LocalTCPAddr() net.Addr { return sc.netConn.LocalAddr() }

// RemoteTCPAddr returns the remote address of the underlying TCP connection.
func (sc *SessionCommon) RemoteTCPAddr() net.Addr { return sc.netConn.RemoteAddr() }

// Ping obtains the round trip latency of the session.
func (sc *SessionCommon) Ping() (time.Duration, error) { return sc.ys.Ping() }

// Close closes the session.
func (sc *SessionCommon) Close() error {
	if sc == nil {
		return nil
	}
	err := sc.ys.Close()
	sc.rMx.Lock()
	sc.nMap = nil
	sc.rMx.Unlock()
	return err
}
