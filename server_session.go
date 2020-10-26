package dmsg

import (
	"fmt"
	"io"
	"net"

	"github.com/skycoin/dmsg/encodedecoder"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/yamux"

	"github.com/skycoin/dmsg/netutil"
	"github.com/skycoin/dmsg/noise"
	"github.com/skycoin/dmsg/servermetrics"
)

// ServerSession represents a session from the perspective of a dmsg server.
type ServerSession struct {
	*SessionCommon
	m servermetrics.Metrics
}

func makeServerSession(m servermetrics.Metrics, entity *EntityCommon, conn net.Conn,
	encrypt bool, ed encodedecoder.EncodeDecoder) (ServerSession, error) {
	var sSes ServerSession
	sSes.SessionCommon = new(SessionCommon)
	sSes.ed = ed
	sSes.nMap = make(noise.NonceMap)
	if err := sSes.SessionCommon.initServer(entity, conn); err != nil {
		m.RecordSession(servermetrics.DeltaFailed) // record failed connection
		return sSes, err
	}
	sSes.m = m
	sSes.encrypt = encrypt
	return sSes, nil
}

// Close implements io.Closer
func (ss *ServerSession) Close() error {
	if ss == nil {
		return nil
	}
	return ss.SessionCommon.Close()
}

// Serve serves the session.
func (ss *ServerSession) Serve() {
	ss.m.RecordSession(servermetrics.DeltaConnect)          // record successful connection
	defer ss.m.RecordSession(servermetrics.DeltaDisconnect) // record disconnection

	for {
		yStr, err := ss.ys.AcceptStream()
		if err != nil {
			switch err {
			case yamux.ErrSessionShutdown, io.EOF:
				ss.log.WithError(err).Info("Stopping session...")
			default:
				ss.log.WithError(err).Warn("Failed to accept stream, stopping session...")
			}
			return
		}

		log := ss.log.WithField("yamux_id", yStr.StreamID())
		log.Info("Initiating stream.")

		go func(yStr *yamux.Stream) {
			err := ss.serveStream(log, yStr)
			log.WithError(err).Info("Stopped stream.")
		}(yStr)
	}
}

func (ss *ServerSession) serveStream(log logrus.FieldLogger, yStr *yamux.Stream) error {
	readRequest := func() (StreamRequest, error) {
		obj, err := ss.readObject(yStr)
		if err != nil {
			return StreamRequest{}, err
		}
		fmt.Println("READ OBJECT")
		req, err := obj.ObtainStreamRequest(ss.ed, ss.encrypt)
		if err != nil {
			return StreamRequest{}, err
		}
		fmt.Println("OBTAINED STREAM REQUEST")
		// TODO(evanlinjin): Implement timestamp tracker.
		if err := req.Verify(0, ss.encrypt); err != nil {
			return StreamRequest{}, err
		}
		fmt.Println("VERIFIED REQUEST")
		if req.SrcAddr.PK != ss.rPK {
			return StreamRequest{}, ErrReqInvalidSrcPK
		}
		fmt.Println("FINISHED readRequest")
		return req, nil
	}

	// Read request.
	req, err := readRequest()
	if err != nil {
		ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
		return err
	}

	log = log.
		WithField("src_addr", req.SrcAddr).
		WithField("dst_addr", req.DstAddr)

	log.Debug("Read stream request from initiating side.")

	// Obtain next session.
	ss2, ok := ss.entity.serverSession(req.DstAddr.PK)
	if !ok {
		ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
		return ErrReqNoNextSession
	}
	log.Debug("Obtained next session.")

	// Forward request and obtain/check response.
	yStr2, resp, err := ss2.forwardRequest(req)
	if err != nil {
		ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
		return err
	}
	log.Debug("Forwarded stream request.")

	// Forward response.
	if err := ss.writeObject(yStr, resp); err != nil {
		ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
		return err
	}
	log.Debug("Forwarded stream response.")

	// Serve stream.
	log.Info("Serving stream.")
	ss.m.RecordStream(servermetrics.DeltaConnect)          // record successful stream
	defer ss.m.RecordStream(servermetrics.DeltaDisconnect) // record disconnection
	return netutil.CopyReadWriteCloser(yStr, yStr2)
}

func (ss *ServerSession) forwardRequest(req StreamRequest) (yStr *yamux.Stream, respObj SignedObject, err error) {
	defer func() {
		if err != nil && yStr != nil {
			ss.log.
				WithError(yStr.Close()).
				Debugf("After forwardRequest failed, the yamux stream is closed.")
		}
	}()

	if yStr, err = ss.ys.OpenStream(); err != nil {
		return nil, nil, err
	}
	fmt.Println("FWD: OPENED STREAM")
	if err = ss.writeObject(yStr, req.raw); err != nil {
		return nil, nil, err
	}
	fmt.Println("FWD: WROTE OBJECT")
	if respObj, err = ss.readObject(yStr); err != nil {
		return nil, nil, err
	}
	fmt.Println("FWD: READ OBJECT")
	var resp StreamResponse
	if resp, err = respObj.ObtainStreamResponse(ss.ed, ss.encrypt); err != nil {
		return nil, nil, err
	}
	fmt.Println("FWD: OBTAINED STREAM RESPONSE")
	if err = resp.Verify(req, ss.encrypt); err != nil {
		return nil, nil, err
	}
	fmt.Println("FWD: VERIFIED RESPONSE")
	return yStr, respObj, nil
}
