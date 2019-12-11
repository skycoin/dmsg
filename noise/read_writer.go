package noise

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/ioutil"
)

// Frame format: [ len (2 bytes) | auth (16 bytes) | payload (<= maxPayloadSize bytes) ]
const (
	maxFrameSize   = 4096                                 // maximum frame size (4096)
	maxPayloadSize = maxFrameSize - prefixSize - authSize // maximum payload size
	maxPrefixValue = maxFrameSize - prefixSize            // maximum value contained in the 'len' prefix

	prefixSize = 2  // len prefix size
	authSize   = 16 // noise auth data size
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "deadline exceeded" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

type netError struct{ Err error }

func (e *netError) Error() string { return e.Err.Error() }
func (netError) Timeout() bool    { return false }
func (netError) Temporary() bool  { return true }

// ReadWriter implements noise encrypted read writer.
type ReadWriter struct {
	origin io.ReadWriter
	ns     *Noise

	rawInput *bufio.Reader
	input    bytes.Buffer

	rMx sync.Mutex
	wMx sync.Mutex
}

// NewReadWriter constructs a new ReadWriter.
func NewReadWriter(rw io.ReadWriter, ns *Noise) *ReadWriter {
	return &ReadWriter{
		origin:   rw,
		ns:       ns,
		rawInput: bufio.NewReaderSize(rw, maxFrameSize*2), // can fit 2 frames.
	}
}

func (rw *ReadWriter) Read(p []byte) (int, error) {
	rw.rMx.Lock()
	defer rw.rMx.Unlock()

	if rw.input.Len() > 0 {
		return rw.input.Read(p)
	}
	ciphertext, err := ReadRawFrame(rw.rawInput)
	if err != nil {
		return 0, err
	}
	plaintext, err := rw.ns.DecryptUnsafe(ciphertext)
	if err != nil {
		return 0, &netError{Err: err}
	}
	if len(plaintext) == 0 {
		return 0, nil
	}
	return ioutil.BufRead(&rw.input, plaintext, p)
}

func (rw *ReadWriter) Write(p []byte) (n int, err error) {
	rw.wMx.Lock()
	defer rw.wMx.Unlock()

	// Enforce max write size.
	if len(p) > maxPayloadSize {
		p, err = p[:maxPayloadSize], io.ErrShortWrite
	}
	if err := WriteRawFrame(rw.origin, rw.ns.EncryptUnsafe(p)); err != nil {
		return 0, err
	}
	return len(p), err
}

// Handshake performs a Noise handshake using the provided io.ReadWriter.
func (rw *ReadWriter) Handshake(hsTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		if rw.ns.init {
			errCh <- InitiatorHandshake(rw.ns, rw.rawInput, rw.origin)
		} else {
			errCh <- ResponderHandshake(rw.ns, rw.rawInput, rw.origin)
		}
		close(errCh)
	}()
	select {
	case err := <-errCh:
		return err
	case <-time.After(hsTimeout):
		return timeoutError{}
	}
}

// LocalStatic returns the local static public key.
func (rw *ReadWriter) LocalStatic() cipher.PubKey {
	return rw.ns.LocalStatic()
}

// RemoteStatic returns the remote static public key.
func (rw *ReadWriter) RemoteStatic() cipher.PubKey {
	return rw.ns.RemoteStatic()
}

// InitiatorHandshake performs a noise handshake as an initiator.
func InitiatorHandshake(ns *Noise, r *bufio.Reader, w io.Writer) error {
	for {
		msg, err := ns.MakeHandshakeMessage()
		if err != nil {
			return err
		}
		if err := WriteRawFrame(w, msg); err != nil {
			return err
		}
		if ns.HandshakeFinished() {
			break
		}
		res, err := ReadRawFrame(r)
		if err != nil {
			return err
		}
		if err = ns.ProcessHandshakeMessage(res); err != nil {
			return err
		}
		if ns.HandshakeFinished() {
			break
		}
	}
	return nil
}

// ResponderHandshake performs a noise handshake as a responder.
func ResponderHandshake(ns *Noise, r *bufio.Reader, w io.Writer) error {
	for {
		msg, err := ReadRawFrame(r)
		if err != nil {
			return err
		}
		if err := ns.ProcessHandshakeMessage(msg); err != nil {
			return err
		}
		if ns.HandshakeFinished() {
			break
		}
		res, err := ns.MakeHandshakeMessage()
		if err != nil {
			return err
		}
		if err := WriteRawFrame(w, res); err != nil {
			return err
		}
		if ns.HandshakeFinished() {
			break
		}
	}
	return nil
}

// WriteRawFrame writes a raw frame (data prefixed with a uint16 len).
func WriteRawFrame(w io.Writer, p []byte) error {
	buf := make([]byte, prefixSize+len(p))
	binary.BigEndian.PutUint16(buf, uint16(len(p)))
	copy(buf[prefixSize:], p)
	_, err := w.Write(buf)
	return err
}

// ReadRawFrame attempts to read a raw frame from a buffered reader.
func ReadRawFrame(r *bufio.Reader) (p []byte, err error) {
	prefixB, err := r.Peek(prefixSize)
	if err != nil {
		return nil, err
	}

	// obtain payload size
	prefix := int(binary.BigEndian.Uint16(prefixB))
	if prefix > maxPrefixValue {
		return nil, &netError{
			Err: fmt.Errorf("noise prefix value %dB exceeds maximum %dB", prefix, maxPrefixValue),
		}
	}

	// obtain payload
	b, err := r.Peek(prefixSize + prefix)
	if err != nil {
		return nil, err
	}
	if _, err := r.Discard(prefixSize + prefix); err != nil {
		panic(fmt.Errorf("unexpected error when discarding %d bytes: %v", prefixSize+prefix, err))
	}
	return b[prefixSize:], nil
}
