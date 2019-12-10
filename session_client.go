package dmsg

import (
	"fmt"
	"net"
	"time"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/netutil"
)

// ClientSession represents a session from the perspective of a dmsg client.
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

// DialStream attempts to dial a stream to a remote client via the dsmg server that this session is connected to.
func (cs *ClientSession) DialStream(dst Addr) (dStr *Stream, err error) {

	var (
		writeDone = false
		readDone  = false
	)

	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to dial stream [write(%v), read(%v)]: %v", writeDone, readDone, err)
		}
	}()

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

	// Do stream handshake.
	req, err := dStr.writeRequest(dst)
	if err != nil {
		return nil, err
	}
	writeDone = true

	if err := dStr.readResponse(req); err != nil {
		return nil, err
	}
	readDone = true

	// Clear deadline.
	if err = dStr.SetDeadline(time.Time{}); err != nil {
		return
	}

	return dStr, err
}

// Serve accepts incoming streams from remote clients.
func (cs *ClientSession) Serve() error {
	defer func() { _ = cs.Close() }() //nolint:errcheck
	for {
		if _, err := cs.acceptStream(); err != nil {
			cs.log.WithError(err).Warn("Stopped accepting streams.")
			return err
		}
	}
}

func (cs *ClientSession) acceptStream() (dStr *Stream, err error) {
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

	// Do stream handshake.
	var req StreamDialRequest
	if req, err = dStr.readRequest(); err != nil {
		return nil, err
	}
	if err = dStr.writeResponse(req); err != nil {
		return nil, err
	}

	// Clear deadline.
	if err = dStr.SetDeadline(time.Time{}); err != nil {
		return
	}

	return dStr, err
}
