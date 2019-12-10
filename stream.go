package dmsg

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/SkycoinProject/yamux"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/noise"
)

// Stream represents a dmsg connection between two dmsg clients.
type Stream struct {
	ses  *ClientSession // back reference
	yStr *yamux.Stream

	rMx sync.Mutex
	wMx sync.Mutex

	// The following fields are to be filled after handshake.
	lAddr  Addr
	rAddr  Addr
	ns     *noise.Noise
	nsConn *noise.ReadWriter
	close  func() // to be called when closing
	log    logrus.FieldLogger
}

func newInitiatingStream(cSes *ClientSession) (*Stream, error) {
	yStr, err := cSes.ys.OpenStream()
	if err != nil {
		return nil, err
	}
	return &Stream{ses: cSes, yStr: yStr}, nil
}

func newRespondingStream(cSes *ClientSession) (*Stream, error) {
	yStr, err := cSes.ys.AcceptStream()
	if err != nil {
		return nil, err
	}
	return &Stream{ses: cSes, yStr: yStr}, nil
}

// Close closes the dmsg stream.
func (s *Stream) Close() error {
	if s == nil {
		return nil
	}
	if s.close != nil {
		s.close()
	}
	return s.yStr.Close()
}

func (s *Stream) writeRequest(rAddr Addr) (req StreamDialRequest, err error) {
	// Reserve stream in porter.
	var lPort uint16
	if lPort, s.close, err = s.ses.porter.ReserveEphemeral(context.Background(), s); err != nil {
		return
	}

	// Prepare fields.
	s.prepareFields(true, Addr{PK: s.ses.LocalPK(), Port: lPort}, rAddr)

	// Prepare request.
	var nsMsg []byte
	if nsMsg, err = s.ns.MakeHandshakeMessage(); err != nil {
		return
	}
	req = StreamDialRequest{
		Timestamp: time.Now().UnixNano(),
		SrcAddr:   s.lAddr,
		DstAddr:   s.rAddr,
		NoiseMsg:  nsMsg,
	}
	req.Sign(s.ses.localSK())

	// Write request.
	err = writeEncryptedGob(s.yStr, &s.wMx, s.ses.ns, req)
	return
}

func (s *Stream) readRequest() (req StreamDialRequest, err error) {
	if err = readEncryptedGob(s.yStr, &s.rMx, s.ses.ns, &req); err != nil {
		return
	}
	if err = req.Verify(0); err != nil {
		err = ErrReqInvalidTimestamp
		return
	}
	if req.DstAddr.PK != s.ses.LocalPK() {
		err = ErrReqInvalidDstPK
		return
	}

	// Prepare fields.
	s.prepareFields(false, req.DstAddr, req.SrcAddr)

	if err = s.ns.ProcessHandshakeMessage(req.NoiseMsg); err != nil {
		return
	}
	return
}

func (s *Stream) writeResponse(req StreamDialRequest) error {
	// Obtain associated local listener.
	pVal, ok := s.ses.porter.PortValue(s.lAddr.Port)
	if !ok {
		return ErrReqNoListener
	}
	lis, ok := pVal.(*Listener)
	if !ok {
		return ErrReqNoListener
	}

	// Prepare and write response.
	nsMsg, err := s.ns.MakeHandshakeMessage()
	if err != nil {
		return err
	}
	resp := StreamDialResponse{
		ReqHash:  req.Hash(),
		Accepted: true,
		NoiseMsg: nsMsg,
	}
	resp.Sign(s.ses.localSK())
	if err := writeEncryptedGob(s.yStr, &s.wMx, s.ses.ns, resp); err != nil {
		return err
	}

	// Push stream to listener.
	return lis.introduceStream(s)
}

func (s *Stream) readResponse(req StreamDialRequest) (err error) {
	// Read and process response.
	var resp StreamDialResponse
	if err = readEncryptedGob(s.yStr, &s.rMx, s.ses.ns, &resp); err != nil {
		return
	}
	if err = resp.Verify(req.DstAddr.PK, req.Hash()); err != nil {
		return
	}
	if err = s.ns.ProcessHandshakeMessage(resp.NoiseMsg); err != nil {
		return
	}

	// Finalize noise read writer.
	s.nsConn = noise.NewReadWriter(s.yStr, s.ns)
	return
}

func (s *Stream) prepareFields(init bool, lAddr, rAddr Addr) {
	ns, err := noise.New(noise.HandshakeKK, noise.Config{
		LocalPK:   s.ses.LocalPK(),
		LocalSK:   s.ses.localSK(),
		RemotePK:  rAddr.PK,
		Initiator: init,
	})
	if err != nil {
		s.log.WithError(err).Panic("Failed to prepare stream noise object.")
	}

	s.lAddr = lAddr
	s.rAddr = rAddr
	s.ns = ns
	s.log = s.ses.log.WithField("stream", s.lAddr.ShortString()+"->"+s.rAddr.ShortString())
}

// LocalAddr returns the local address of the dmsg stream.
func (s *Stream) LocalAddr() net.Addr {
	return s.lAddr
}

// RemoteAddr returns the remote address of the dmsg stream.
func (s *Stream) RemoteAddr() net.Addr {
	return s.rAddr
}

// StreamID returns the stream ID.
func (s *Stream) StreamID() uint32 {
	return s.yStr.StreamID()
}

// Read implements io.Reader
func (s *Stream) Read(b []byte) (int, error) {
	return s.yStr.Read(b)
}

// Write implements io.Writer
func (s *Stream) Write(b []byte) (int, error) {
	return s.yStr.Write(b)
}

// SetDeadline implements net.Conn
func (s *Stream) SetDeadline(t time.Time) error {
	return s.yStr.SetDeadline(t)
}

// SetReadDeadline implements net.Conn
func (s *Stream) SetReadDeadline(t time.Time) error {
	return s.yStr.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn
func (s *Stream) SetWriteDeadline(t time.Time) error {
	return s.yStr.SetWriteDeadline(t)
}
