package dmsg

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"github.com/SkycoinProject/dmsg/netutil"
	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"
	"io"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/noise"
)

// Session handles the multiplexed connection between the dmsg server and dmsg client.
type Session struct {
	lPK cipher.PubKey
	lSK cipher.SecKey
	rPK cipher.PubKey // Public key of the remote dmsg server.

	ys     *yamux.Session
	ns     *noise.Noise    // For encrypting session messages, not stream messages.
	porter *netutil.Porter

	log logrus.FieldLogger
}

func (s *Session) DialStream(ctx context.Context, dst Addr) (ds *Stream2, err error) {
	done := make(chan struct{})
	defer close(done)

	// Prepare yamux stream.
	ys, err := s.ys.OpenStream()
	if err != nil {
		return nil, err
	}
	go func() {
		select {
		case <-ctx.Done():
			s.log.
				WithError(ys.Close()).
				Warnf("failed to dial stream: %v", ctx.Err())
		case <-done:
			if err != nil {
				if closeErr := ys.Close(); closeErr != nil {
					s.log.
						WithError(closeErr).
						Debug("stream closed with error")
				}
			}
			return
		}
	}()

	// Prepare dmsg stream to reserve in porter.
	return NewEphemeralStream(ctx, s.log, s.porter, s.ns, ys, s.lPK, s.lSK, dst)
}

func (s *Session) AcceptStream(ctx context.Context) (ds *Stream2, err error) {
	done := make(chan struct{})
	defer close(done)

	ys, err := s.ys.AcceptStream()
	if err != nil {
		return nil, err
	}
	go func() {

	}()

	var req StreamDialRequest
	if err := readEncryptedGob(ys, s.ns, &req); err != nil {
		return nil, err
	}
	// TODO(evanlinjin): Create TimestampTracker.
	if err := req.Verify(0); err != nil {
		return nil, err
	}
	lv, ok := s.porter.PortValue(req.DstAddr.Port)
	if !ok {
		return nil, ErrIncomingHasNoListener
	}
	l, ok := lv.(*Listener)
	if !ok {
		return nil, ErrIncomingHasNoListener
	}

	// TODO(evanlinjin): Finish handshake before pushing to listener.
	if err = l.IntroduceStream(nil); err != nil {
		return nil, err
	}
	return nil, nil
}

// TODO(evanlinjin): Complete this!
func watchStream(ctx context.Context, done chan struct{}, log logrus.FieldLogger, err *error, closeFunc func() error) {
	select {
	case <-ctx.Done():

	}
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
	binary.BigEndian.PutUint16(p, uint16(len(p) - 2))
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
