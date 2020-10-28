package dmsg

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/yamux"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/noise"
)

// Stream represents a dmsg connection between two dmsg clients.
type Stream struct {
	ses  *ClientSession // back reference
	yStr *yamux.Stream

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

// Logger returns the internal logrus.FieldLogger instance.
func (s *Stream) Logger() logrus.FieldLogger {
	return s.log
}

func (s *Stream) writeRequest(rAddr Addr) (req StreamRequest, err error) {
	fmt.Printf("REMOTE DMSG ADDR: %v\n", rAddr)
	// Reserve stream in porter.
	var lPort uint16
	if lPort, s.close, err = s.ses.porter.ReserveEphemeral(context.Background(), s); err != nil {
		return
	}

	// Prepare fields.
	s.prepareFields(true, Addr{PK: s.ses.LocalPK(), Port: lPort}, rAddr)

	req = StreamRequest{
		Timestamp: time.Now().UnixNano(),
		SrcAddr:   s.lAddr,
		DstAddr:   s.rAddr,
	}

	var obj SignedObject
	if s.ses.encrypt {
		// Prepare request.
		var nsMsg []byte
		if nsMsg, err = s.ns.MakeHandshakeMessage(); err != nil {
			return
		}
		req.NoiseMsg = nsMsg
		obj = MakeSignedStreamRequest(s.ses.ed, &req, s.ses.localSK())
	} else {
		obj = s.ses.ed.Encode(&req)
	}

	// Write request.
	err = s.ses.writeObject(s.yStr, obj)
	return
}

func (s *Stream) readRequest() (req StreamRequest, err error) {
	var obj SignedObject
	if obj, err = s.ses.readObject(s.yStr); err != nil {
		return
	}
	fmt.Println("READREQ: READ OBJECT")
	if req, err = obj.ObtainStreamRequest(s.ses.ed, s.ses.encrypt); err != nil {
		return
	}
	fmt.Println("READREQ: OBTAINED STREAM REQUEST")
	if err = req.Verify(0, s.ses.encrypt); err != nil {
		return
	}
	fmt.Println("READREQ: VERIFIED STREAM REQUEST")
	if req.DstAddr.PK != s.ses.LocalPK() {
		err = ErrReqInvalidDstPK
		return
	}

	// Prepare fields.
	s.prepareFields(false, req.DstAddr, req.SrcAddr)

	if s.ses.encrypt {
		if err = s.ns.ProcessHandshakeMessage(req.NoiseMsg); err != nil {
			return
		}
	}

	return
}

func (s *Stream) writeResponse(reqHash cipher.SHA256) error {
	// Obtain associated local listener.
	fmt.Printf("WRITE RESP: GETTING VALUE FROM PORTER FOR %d\n", s.lAddr.Port)
	pVal, ok := s.ses.porter.PortValue(s.lAddr.Port)
	if !ok {
		fmt.Println("WRITE RESP: NO VAL IN PORTER")
		return ErrReqNoListener
	}
	lis, ok := pVal.(*Listener)
	if !ok {
		fmt.Printf("WRITE RESP: VAL IN PORTER IS NOT LISTENER, OF TYPE: %v\n", reflect.TypeOf(pVal))
		return ErrReqNoListener
	}

	resp := StreamResponse{
		ReqHash:  reqHash,
		Accepted: true,
	}

	var obj SignedObject
	if s.ses.encrypt {
		// Prepare and write response.
		nsMsg, err := s.ns.MakeHandshakeMessage()
		if err != nil {
			return err
		}
		resp.NoiseMsg = nsMsg
		obj = MakeSignedStreamResponse(s.ses.ed, &resp, s.ses.localSK())
	} else {
		obj = s.ses.ed.Encode(&resp)
	}

	if err := s.ses.writeObject(s.yStr, obj); err != nil {
		return err
	}

	// Push stream to listener.
	return lis.introduceStream(s)
}

func (s *Stream) readResponse(req StreamRequest) error {
	obj, err := s.ses.readObject(s.yStr)
	if err != nil {
		return err
	}
	fmt.Println("READRESP: READ OBJECT")
	resp, err := obj.ObtainStreamResponse(s.ses.ed, s.ses.encrypt)
	if err != nil {
		return err
	}
	fmt.Println("READRESP: OBTAINED STREAM RESPONSE")
	if err := resp.Verify(req, s.ses.encrypt); err != nil {
		return err
	}
	fmt.Println("READRESP: VERIFIED RESPONSE")

	if s.ses.encrypt {
		return s.ns.ProcessHandshakeMessage(resp.NoiseMsg)
	}

	return nil
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

	fmt.Printf("PREP FIELDS: SETTING L ADDR TO %v\n", lAddr)
	s.lAddr = lAddr
	s.rAddr = rAddr
	s.ns = ns
	s.nsConn = noise.NewReadWriter(s.yStr, s.ns)
	s.log = s.ses.log.WithField("stream", s.lAddr.ShortString()+"->"+s.rAddr.ShortString())
}

// LocalAddr returns the local address of the dmsg stream.
func (s *Stream) LocalAddr() net.Addr {
	return s.lAddr
}

// RawLocalAddr returns the local address as dmsg.Addr type.
func (s *Stream) RawLocalAddr() Addr {
	return s.lAddr
}

// RemoteAddr returns the remote address of the dmsg stream.
func (s *Stream) RemoteAddr() net.Addr {
	return s.rAddr
}

// RawRemoteAddr returns the remote address as dmsg.Addr type.
func (s *Stream) RawRemoteAddr() Addr {
	return s.rAddr
}

// ServerPK returns the remote PK of the dmsg.Server used to relay frames to and from the remote client.
func (s *Stream) ServerPK() cipher.PubKey {
	return s.ses.RemotePK()
}

// StreamID returns the stream ID.
func (s *Stream) StreamID() uint32 {
	return s.yStr.StreamID()
}

// Read implements io.Reader
func (s *Stream) Read(b []byte) (int, error) {
	fmt.Println("STREAM READ: READING")

	if s.ses.encrypt {
		return s.nsConn.Read(b)
	}

	return s.yStr.Read(b)
}

// Write implements io.Writer
func (s *Stream) Write(b []byte) (int, error) {
	if s.ses.encrypt {
		return s.nsConn.Write(b)
	}

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
