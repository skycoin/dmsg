package dmsg

import (
	"net"
	"time"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/netutil"
)

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

func (cs *ClientSession) DialStream(dst Addr) (dStr *Stream2, err error) {
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

func (cs *ClientSession) Serve() error {
	for {
		if _, err := cs.acceptStream(); err != nil {
			return err
		}
	}
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
