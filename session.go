package dmsg

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"io"

	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/noise"
)

// Session handles the multiplexed connection between the dmsg server and dmsg client.
type Session struct {
	lPK cipher.PubKey
	rPK cipher.PubKey // Public key of the remote dmsg server.
	sk  cipher.SecKey
	ys  *yamux.Session
	ns  *noise.Noise
	log logrus.FieldLogger
}

func (s *Session) DialStream(ctx context.Context, req StreamDialRequest) (conn *Stream2, err error) {
	var stream *yamux.Stream
	if stream, err = s.ys.OpenStream(); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if err := stream.Close(); err != nil {
				s.log.WithError(err).Debug("stream closed with error")
			}
		}
	}()
	if err := encryptedGobEncode(stream, s.ns, req); err != nil {
		return nil, err
	}
	var resp DialResponse
	if err := encryptedGobDecode(stream, s.ns, resp); err != nil {
		return nil, err
	}
	if err = resp.Verify(req.DstAddr.PK, req.Hash()); err != nil {
		return nil, err
	}

	// TODO(evanlinjin): Figure this out.
	ns, err := noise.New(noise.HandshakeXK, noise.Config{

	})

	conn = &Stream2{
		lAddr: req.SrcAddr,
		rAddr: req.DstAddr,
		sk:    s.sk,
		ys:    stream,
		ns:    ns,
	}

	return conn, nil
}

func gobEncode(v interface{}) []byte {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(v); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// gobEncode encrypted with noise and prefixed with uint16 (2 additional bytes).
func encryptedGobEncode(w io.Writer, ns *noise.Noise, v interface{}) error {
	p := ns.EncryptUnsafe(gobEncode(v))
	p = append(make([]byte, 2), p...)
	binary.BigEndian.PutUint16(p, uint16(len(p) - 2))
	_, err := w.Write(p)
	return err
}

func gobDecode(v interface{}, b []byte) error {
	return gob.NewDecoder(bytes.NewReader(b)).Decode(v)
}

func encryptedGobDecode(r io.Reader, ns *noise.Noise, v interface{}) error {
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
	return gobDecode(v, b)
}
