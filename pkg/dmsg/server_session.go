// Package dmsg pkg/dmsg/server_session.go
package dmsg

import (
	"fmt"
	"io"
	"net"

	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/netutil"
	"github.com/xtaci/smux"

	"github.com/skycoin/dmsg/internal/servermetrics"
	"github.com/skycoin/dmsg/pkg/noise"
)

// ServerSession represents a session from the perspective of a dmsg server.
type ServerSession struct {
	*SessionCommon
	m servermetrics.Metrics
}

func makeServerSession(m servermetrics.Metrics, entity *EntityCommon, conn net.Conn) (ServerSession, error) {
	var sSes ServerSession
	sSes.SessionCommon = new(SessionCommon)
	sSes.nMap = make(noise.NonceMap)
	if err := sSes.SessionCommon.initServer(entity, conn); err != nil {
		m.RecordSession(servermetrics.DeltaFailed) // record failed connection
		return sSes, err
	}
	sSes.m = m
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
	if ss.ss != nil {
		for {
			sStr, err := ss.ss.AcceptStream()
			if err != nil {
				switch err {
				case io.EOF:
					ss.log.WithError(err).Info("Stopping session...")
				default:
					ss.log.WithError(err).Warn("Failed to accept stream, stopping session...")
				}
				return
			}

			log := ss.log.WithField("smux_id", sStr.ID())
			log.Info("Initiating stream.")

			go func(sStr *smux.Stream) {
				err := ss.serveStreamSmux(log, sStr, ss.ss.RemoteAddr())
				log.WithError(err).Info("Stopped stream.")
			}(sStr)
		}
	} else {
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
				err := ss.serveStreamYamux(log, yStr, ss.ys.RemoteAddr())
				log.WithError(err).Info("Stopped stream.")
			}(yStr)
		}
	}
}

// struct

func (ss *ServerSession) serveStreamYamux(log logrus.FieldLogger, yStr *yamux.Stream, addr net.Addr) error {
	readRequest := func() (StreamRequest, error) {
		obj, err := ss.readObject(yStr)
		if err != nil {
			return StreamRequest{}, err
		}
		req, err := obj.ObtainStreamRequest()
		if err != nil {
			return StreamRequest{}, err
		}
		// TODO(evanlinjin): Implement timestamp tracker.
		if err := req.Verify(0); err != nil {
			return StreamRequest{}, err
		}
		if req.SrcAddr.PK != ss.rPK {
			return StreamRequest{}, ErrReqInvalidSrcPK
		}
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
	if req.IPinfo && req.DstAddr.PK == ss.entity.LocalPK() {
		log.Debug("Received IP stream request.")

		ip, err := addrToIP(addr)
		if err != nil {
			ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
			return err
		}

		resp := StreamResponse{
			ReqHash:  req.raw.Hash(),
			Accepted: true,
			IP:       ip,
		}
		obj := MakeSignedStreamResponse(&resp, ss.entity.LocalSK())

		if err := ss.writeObject(yStr, obj); err != nil {
			ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
			return err
		}
		log.Debug("Wrote IP stream response.")
		return nil
	}

	// Obtain next session.
	ss2, ok := ss.entity.serverSession(req.DstAddr.PK)
	if !ok {
		ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
		return ErrReqNoNextSession
	}
	log.Debug("Obtained next session.")

	// Forward request and obtain/check response.
	yStr2, resp, err := ss2.forwardRequestYamux(req)
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

func (ss *ServerSession) serveStreamSmux(log logrus.FieldLogger, sStr *smux.Stream, addr net.Addr) error {
	readRequest := func() (StreamRequest, error) {
		obj, err := ss.readObject(sStr)
		if err != nil {
			return StreamRequest{}, err
		}
		req, err := obj.ObtainStreamRequest()
		if err != nil {
			return StreamRequest{}, err
		}
		// TODO(evanlinjin): Implement timestamp tracker.
		if err := req.Verify(0); err != nil {
			return StreamRequest{}, err
		}
		if req.SrcAddr.PK != ss.rPK {
			return StreamRequest{}, ErrReqInvalidSrcPK
		}
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
	if req.IPinfo && req.DstAddr.PK == ss.entity.LocalPK() {
		log.Debug("Received IP stream request.")

		ip, err := addrToIP(addr)
		if err != nil {
			ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
			return err
		}

		resp := StreamResponse{
			ReqHash:  req.raw.Hash(),
			Accepted: true,
			IP:       ip,
		}
		obj := MakeSignedStreamResponse(&resp, ss.entity.LocalSK())

		if err := ss.writeObject(sStr, obj); err != nil {
			ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
			return err
		}
		log.Debug("Wrote IP stream response.")
		return nil
	}

	// Obtain next session.
	ss2, ok := ss.entity.serverSession(req.DstAddr.PK)
	if !ok {
		ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
		return ErrReqNoNextSession
	}
	log.Debug("Obtained next session.")

	// Forward request and obtain/check response.
	sStr2, resp, err := ss2.forwardRequestSmux(req)
	if err != nil {
		ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
		return err
	}
	log.Debug("Forwarded stream request.")

	// Forward response.
	if err := ss.writeObject(sStr, resp); err != nil {
		ss.m.RecordStream(servermetrics.DeltaFailed) // record failed stream
		return err
	}
	log.Debug("Forwarded stream response.")

	// Serve stream.
	log.Info("Serving stream.")
	ss.m.RecordStream(servermetrics.DeltaConnect)          // record successful stream
	defer ss.m.RecordStream(servermetrics.DeltaDisconnect) // record disconnection
	return netutil.CopyReadWriteCloser(sStr, sStr2)
}

func addrToIP(addr net.Addr) (net.IP, error) {
	switch a := addr.(type) {
	case *net.TCPAddr:
		return a.IP, nil
	case *net.UDPAddr:
		return a.IP, nil
	default:
		return nil, fmt.Errorf("unsupported address type %T", addr)
	}
}

func (ss *ServerSession) forwardRequestYamux(req StreamRequest) (yStr *yamux.Stream, respObj SignedObject, err error) {
	defer func() {
		if err != nil && yStr != nil {
			ss.log.
				WithError(yStr.Close()).
				Debugf("After forwardRequest failed, the yamux stream is closed.")
		}
	}()
	fmt.Println("here 1")
	if yStr, err = ss.ys.OpenStream(); err != nil {
		return nil, nil, err
	}
	fmt.Println("here 2")
	if err = ss.writeObject(yStr, req.raw); err != nil {
		return nil, nil, err
	}
	fmt.Println("here 3")
	if respObj, err = ss.readObject(yStr); err != nil {
		return nil, nil, err
	}
	fmt.Println("here 4")
	var resp StreamResponse
	if resp, err = respObj.ObtainStreamResponse(); err != nil {
		return nil, nil, err
	}
	fmt.Println("here 5")
	if err = resp.Verify(req); err != nil {
		return nil, nil, err
	}
	fmt.Println("here 6")
	return yStr, respObj, nil
}

func (ss *ServerSession) forwardRequestSmux(req StreamRequest) (sStr *smux.Stream, respObj SignedObject, err error) {
	defer func() {
		if err != nil && sStr != nil {
			ss.log.
				WithError(sStr.Close()).
				Debugf("After forwardRequest failed, the yamux stream is closed.")
		}
	}()
	fmt.Println("here 1")
	if sStr, err = ss.ss.OpenStream(); err != nil {
		return nil, nil, err
	}
	fmt.Println("here 2")
	if err = ss.writeObject(sStr, req.raw); err != nil {
		return nil, nil, err
	}
	fmt.Println("here 3")
	if respObj, err = ss.readObject(sStr); err != nil {
		return nil, nil, err
	}
	fmt.Println("here 4")
	var resp StreamResponse
	if resp, err = respObj.ObtainStreamResponse(); err != nil {
		return nil, nil, err
	}
	fmt.Println("here 5")
	if err = resp.Verify(req); err != nil {
		return nil, nil, err
	}
	fmt.Println("here 6")
	return sStr, respObj, nil
}
