package dmsg

import (
	"bufio"
	"net"

	"github.com/SkycoinProject/yamux"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/noise"
)

type SessionCommon struct {
	entity *EntityCommon // back reference
	rPK    cipher.PubKey // remote pk

	ys *yamux.Session
	ns *noise.Noise

	log logrus.FieldLogger
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
	sc.ys = ySes
	sc.ns = ns
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
	sc.ys = ySes
	sc.ns = ns
	sc.log = entity.log.WithField("session", ns.RemoteStatic())
	return nil
}

func (sc *SessionCommon) localSK() cipher.SecKey { return sc.entity.sk }

func (sc *SessionCommon) LocalPK() cipher.PubKey { return sc.entity.pk }

func (sc *SessionCommon) RemotePK() cipher.PubKey { return sc.rPK }

func (sc *SessionCommon) Close() error {
	return sc.ys.Close()
}
