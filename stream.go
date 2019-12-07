package dmsg

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"io"
	"net"
	"time"

	"github.com/SkycoinProject/dmsg/netutil"

	"github.com/SkycoinProject/yamux"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/noise"
)

type Stream struct {
	lAddr Addr          // local address
	rAddr Addr          // remote address
	sk    cipher.SecKey // Local secret key.

	yStr *yamux.Stream     // Underlying yamux stream.
	ns   *noise.ReadWriter // Underlying noise read writer.

	close func() // to be called when closing.
	log   logrus.FieldLogger
}

func NewStream(ys *yamux.Stream, lSK cipher.SecKey, src, dst Addr) *Stream {
	return &Stream{
		lAddr: src,
		rAddr: dst,
		sk:    lSK,
		yStr:  ys,
	}
}

type ClientStreamHandshake func(ctx context.Context, log logrus.FieldLogger, porter *netutil.Porter, sessionNoise *noise.Noise) error

func (ds *Stream) DoClientHandshake(ctx context.Context, log logrus.FieldLogger, porter *netutil.Porter, sessionNoise *noise.Noise, hs ClientStreamHandshake) (err error) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- hs(ctx, log, porter, sessionNoise)
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errCh:
	}

	if err != nil {
		log.WithError(ds.Close()).
			Warnf("failed to complete handshake for stream: %v", err)
		return err
	}
	return nil
}

func (ds *Stream) ClientInitiatingHandshake(ctx context.Context, log logrus.FieldLogger, porter *netutil.Porter, sessionNoise *noise.Noise) error {
	saveStream := func() error {
		lPort, closeDS, err := porter.ReserveEphemeral(ctx, ds)
		if err != nil {
			return err
		}
		ds.lAddr.Port = lPort
		ds.close = closeDS
		ds.log = log.WithField("stream", ds.lAddr.ShortString()+"->"+ds.rAddr.ShortString())
		return nil
	}

	writeRequest := func(ns *noise.Noise) (*StreamDialRequest, error) {
		nsMsg, err := ns.MakeHandshakeMessage()
		if err != nil {
			return nil, err
		}
		req := StreamDialRequest{
			Timestamp: time.Now().UnixNano(),
			SrcAddr:   ds.lAddr,
			DstAddr:   ds.rAddr,
			NoiseMsg:  nsMsg,
		}
		if err := req.Sign(ds.sk); err != nil {
			return nil, err
		}
		if err := writeEncryptedGob(ds.yStr, sessionNoise, req); err != nil {
			return nil, err
		}
		return &req, nil
	}

	readResponse := func(ns *noise.Noise, req *StreamDialRequest) error {
		var resp DialResponse
		if err := readEncryptedGob(ds.yStr, sessionNoise, &resp); err != nil {
			return err
		}
		if err := resp.Verify(req.DstAddr.PK, req.Hash()); err != nil {
			return err
		}
		return ns.ProcessHandshakeMessage(resp.NoiseMsg)
	}

	log = log.WithField("fn", "ClientInitiatingHandshake")

	// Save stream in porter.
	if err := saveStream(); err != nil {
		return err
	}
	log.Info("Stream saved.")

	// Prepare noise.
	ns, err := ds.prepareNoise(true)
	if err != nil {
		return err
	}
	log.Info("Noise prepared.")

	// Prepare and write request object.
	req, err := writeRequest(ns)
	if err != nil {
		return err
	}
	log.Info("Request written.")

	// Await and read response object.
	if err := readResponse(ns, req); err != nil {
		return err
	}
	log.Info("Response read.")

	// Prepare noise read writer.
	ds.ns = noise.NewReadWriter(ds.yStr, ns)
	return nil
}

func (ds *Stream) ClientRespondingHandshake(_ context.Context, log logrus.FieldLogger, porter *netutil.Porter, sessionNoise *noise.Noise) error {
	readRequest := func() (*StreamDialRequest, error) {
		var req StreamDialRequest
		if err := readEncryptedGob(ds.yStr, sessionNoise, &req); err != nil {
			return nil, err
		}
		if err := req.Verify(0); err != nil { // TODO(evanlinjin): timestamp tracker.
			return nil, ErrReqInvalidTimestamp
		}
		if req.DstAddr.PK != ds.lAddr.PK {
			return nil, ErrReqInvalidDstPK
		}
		ds.lAddr = req.DstAddr
		ds.rAddr = req.SrcAddr
		ds.log = log.WithField("stream", ds.lAddr.ShortString()+"->"+ds.rAddr.ShortString())
		return &req, nil
	}

	checkRequest := func(ns *noise.Noise, req *StreamDialRequest) (*Listener, error) {
		if err := ns.ProcessHandshakeMessage(req.NoiseMsg); err != nil {
			return nil, err
		}
		pv, ok := porter.PortValue(ds.lAddr.Port)
		if !ok {
			return nil, ErrReqNoListener
		}
		lis, ok := pv.(*Listener)
		if !ok {
			return nil, ErrReqNoListener
		}
		return lis, nil
	}

	writeResponse := func(ns *noise.Noise, req *StreamDialRequest) error {
		nsMsg, err := ns.MakeHandshakeMessage()
		if err != nil {
			return err
		}
		resp := DialResponse{
			ReqHash:  req.Hash(),
			Accepted: true,
			NoiseMsg: nsMsg,
		}
		if err := resp.Sign(ds.sk); err != nil {
			return err
		}
		return writeEncryptedGob(ds.yStr, sessionNoise, resp)
	}

	writeReject := func(req *StreamDialRequest, err error) {
		if req == nil {
			return
		}

		resp := DialResponse{
			ReqHash:  req.Hash(),
			Accepted: false,
			ErrCode:  CodeFromError(err),
		}
		if err := resp.Sign(ds.sk); err != nil {
			ds.log.
				WithError(err).
				Error("failed to sign reject response")
		}
		if err := writeEncryptedGob(ds.yStr, sessionNoise, resp); err != nil {
			ds.log.
				WithError(err).
				Error("failed to write reject response")
		}
		ds.log.
			WithError(err).
			WithField("remote", ds.rAddr.ShortString()).
			Warn("rejected stream request")
	}

	log = log.WithField("fn", "ClientRespondingHandshake")

	// Await and read request object.
	req, err := readRequest()
	if err != nil {
		writeReject(req, err)
		return err
	}
	log.Info("Read request.")

	// Prepare noise.
	ns, err := ds.prepareNoise(false)
	if err != nil {
		writeReject(req, err)
		return err
	}

	// Check request and return listener.
	lis, err := checkRequest(ns, req)
	if err != nil {
		writeReject(req, err)
		return err
	}

	// Prepare and write response object.
	if err := writeResponse(ns, req); err != nil {
		return err
	}

	// Prepare noise read writer.
	ds.ns = noise.NewReadWriter(ds.yStr, ns)
	return lis.IntroduceStream(ds)
}

func (ds *Stream) prepareNoise(init bool) (*noise.Noise, error) {
	ns, err := noise.New(noise.HandshakeKK, noise.Config{
		LocalPK:   ds.lAddr.PK,
		LocalSK:   ds.sk,
		RemotePK:  ds.rAddr.PK,
		Initiator: init,
	})
	return ns, err
}

func (ds *Stream) LocalAddr() net.Addr {
	return ds.lAddr
}

func (ds *Stream) RemoteAddr() net.Addr {
	return ds.rAddr
}

func (ds *Stream) StreamID() uint32 {
	return ds.yStr.StreamID()
}

func (ds *Stream) Read(b []byte) (int, error) {
	n, err := ds.ns.Read(b)
	if _, ok := err.(net.Error); err != nil && !ok {
		ds.log.WithError(err).Info("Read() returned error that does not implement net.Error.")
	}
	return n, err
}

func (ds *Stream) Write(b []byte) (int, error) {
	n, err := ds.ns.Write(b)
	if _, ok := err.(net.Error); err != nil && !ok {
		ds.log.WithError(err).Info("Write() returned error that does not implement net.Error.")
	}
	return n, err
}

func (ds *Stream) SetDeadline(t time.Time) error {
	return ds.yStr.SetDeadline(t)
}

func (ds *Stream) SetReadDeadline(t time.Time) error {
	return ds.yStr.SetReadDeadline(t)
}

func (ds *Stream) SetWriteDeadline(t time.Time) error {
	return ds.yStr.SetWriteDeadline(t)
}

func (ds *Stream) Close() error {
	if ds.close != nil {
		ds.close()
	}
	return ds.yStr.Close()
}

func encodeGob(v interface{}) []byte {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(v); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// writeEncryptedGob encrypts with noise and prefixed with uint16 (2 additional bytes).
func writeEncryptedGob(w io.Writer, ns *noise.Noise, v interface{}) error {
	p := ns.EncryptUnsafe(encodeGob(v))
	p = append(make([]byte, 2), p...)
	binary.BigEndian.PutUint16(p, uint16(len(p)-2))
	_, err := w.Write(p)
	return err
}

func decodeGob(v interface{}, b []byte) error {
	return gob.NewDecoder(bytes.NewReader(b)).Decode(v)
}

func readEncryptedGob(r io.Reader, ns *noise.Noise, v interface{}) error {
	lb := make([]byte, 2)
	if _, err := io.ReadFull(r, lb); err != nil {
		return err
	}
	pb := make([]byte, binary.BigEndian.Uint16(lb))
	if _, err := io.ReadFull(r, pb); err != nil {
		return err
	}
	b, err := ns.DecryptUnsafe(pb)
	if err != nil {
		return err
	}
	return decodeGob(v, b)
}
