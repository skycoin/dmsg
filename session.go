package dmsg

import (
	"bufio"
	"net"
	"time"

	"github.com/SkycoinProject/yamux"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/netutil"
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

type ClientSession struct {
	*SessionCommon
	porter *netutil.Porter
}

func makeClientSession(entity *EntityCommon, porter *netutil.Porter, conn net.Conn, rPK cipher.PubKey) (ClientSession, error) {
	var cSes ClientSession
	cSes.SessionCommon = new(SessionCommon)
	if err := cSes.SessionCommon.initClient(entity, conn, rPK); err != nil {
		return cSes, err
	}
	cSes.porter = porter
	return cSes, nil
}

func (cs *ClientSession) dialStream(dst Addr) (dStr *Stream2, err error) {
	if dStr, err = newInitiatingStream(cs); err != nil {
		return nil, err
	}

	// Close stream on failure.
	defer func() {
		if err != nil {
			_ = dStr.Close() //nolint:errcheck
		}
	}()

	// Prepare deadline.
	if err = dStr.SetDeadline(time.Now().Add(HandshakeTimeout)); err != nil {
		return
	}
	defer func() { _ = dStr.SetDeadline(time.Time{}) }() //nolint:errcheck

	// Do stream handshake.
	req, err := dStr.writeRequest(dst)
	if err != nil {
		return nil, err
	}
	if err := dStr.readResponse(req); err != nil {
		return nil, err
	}
	return dStr, err
}

func (cs *ClientSession) acceptStream() (dStr *Stream2, err error) {
	if dStr, err = newRespondingStream(cs); err != nil {
		return nil, err
	}

	// Close stream on failure.
	defer func() {
		if err != nil {
			_ = dStr.Close() //nolint:errcheck
		}
	}()

	// Prepare deadline.
	if err = dStr.SetDeadline(time.Now().Add(HandshakeTimeout)); err != nil {
		return nil, err
	}
	defer func() { _ = dStr.SetDeadline(time.Time{}) }() //nolint:errcheck

	// Do stream handshake.
	var req StreamDialRequest
	if req, err = dStr.readRequest(); err != nil {
		return nil, err
	}
	if err = dStr.writeResponse(req); err != nil {
		return nil, err
	}
	return dStr, err
}

type ServerSession struct {
	*SessionCommon
}

func makeServerSession(entity *EntityCommon, conn net.Conn) (ServerSession, error) {
	var sSes ServerSession
	sSes.SessionCommon = new(SessionCommon)
	if err := sSes.SessionCommon.initServer(entity, conn); err != nil {
		return sSes, err
	}
	return sSes, nil
}

func (ss *ServerSession) acceptAndProxyStream() error {
	yStr, err := ss.ys.AcceptStream()
	if err != nil {
		return err
	}
	go func() {
		err := ss.proxyStream(yStr)
		_ = yStr.Close() //nolint:errcheck
		ss.log.
			WithError(err).
			Infof("acceptAndProxyStream stopped.")
	}()
	return nil
}

func (ss *ServerSession) proxyStream(yStr *yamux.Stream) error {
	readRequest := func() (StreamDialRequest, error) {
		var req StreamDialRequest
		if err := readEncryptedGob(yStr, ss.ns, &req); err != nil {
			return req, err
		}
		if err := req.Verify(0); err != nil { // TODO(evanlinjin): timestamp tracker.
			return req, ErrReqInvalidTimestamp
		}
		if req.SrcAddr.PK != ss.rPK {
			return req, ErrReqInvalidSrcPK
		}
		return req, nil
	}

	log := ss.log.WithField("fn", "proxyStream")

	// Read request.
	req, err := readRequest()
	if err != nil {
		return err
	}
	log.Info("Request read.")

	// Obtain next session.
	log.Infof("attempting to get PK: %s", req.DstAddr.PK)
	ss2, ok := ss.entity.ServerSession(req.DstAddr.PK)
	if !ok {
		return ErrReqNoSession
	}
	log.Info("Next session obtained.")

	// Forward request and obtain/check response.
	yStr2, resp, err := ss2.forwardRequest(req)
	if err != nil {
		return err
	}
	defer func() { _ = yStr2.Close() }() //nolint:errcheck

	// Forward response.
	if err := writeEncryptedGob(yStr, ss.ns, resp); err != nil {
		return err
	}

	// Serve stream.
	return netutil.CopyReadWriter(yStr, yStr2)
}

func (ss *ServerSession) forwardRequest(req StreamDialRequest) (*yamux.Stream, DialResponse, error) {
	yStr, err := ss.ys.OpenStream()
	if err != nil {
		return nil, DialResponse{}, err
	}
	if err := writeEncryptedGob(yStr, ss.ns, req); err != nil {
		_ = yStr.Close() //nolint:errcheck
		return nil, DialResponse{}, err
	}
	var resp DialResponse
	if err := readEncryptedGob(yStr, ss.ns, &resp); err != nil {
		_ = yStr.Close() //nolint:errcheck
		return nil, DialResponse{}, err
	}
	if err := resp.Verify(req.DstAddr.PK, req.Hash()); err != nil {
		_ = yStr.Close() //nolint:errcheck
		return nil, DialResponse{}, err
	}
	return yStr, resp, nil
}
