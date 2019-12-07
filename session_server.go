package dmsg

import (
	"net"

	"github.com/SkycoinProject/yamux"

	"github.com/SkycoinProject/dmsg/netutil"
)

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
