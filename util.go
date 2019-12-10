package dmsg

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"io"
	"sync"

	"github.com/SkycoinProject/dmsg/noise"
)

func awaitDone(ctx context.Context, done chan struct{}) {
	select {
	case <-ctx.Done():
	case <-done:
	}
	return
}

func isClosed(done chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

/* Encrypted IO */

func encodeGob(v interface{}) []byte {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(v); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func decodeGob(v interface{}, b []byte) error {
	return gob.NewDecoder(bytes.NewReader(b)).Decode(v)
}

// writeEncryptedGob encrypts with noise and prefixed with uint16 (2 additional bytes).
func writeEncryptedGob(w io.Writer, mx *sync.Mutex, ns *noise.Noise, v interface{}) error {
	//mx.Lock()
	//defer mx.Unlock()

	p := ns.EncryptUnsafe(encodeGob(v))
	p = append(make([]byte, 2), p...)
	binary.BigEndian.PutUint16(p, uint16(len(p)-2))
	_, err := w.Write(p)
	return err
}

func readEncryptedGob(r io.Reader, mx *sync.Mutex, ns *noise.Noise, v interface{}) error {
	//mx.Lock()
	//defer mx.Unlock()

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
