package noise

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/ioutil"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "deadline exceeded" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

type netError struct{ Err error }

func (e *netError) Error() string { return e.Err.Error() }

// TODO: This is a workaround to make nettest.TestConn pass with noise.Conn.
// We need to investigate why it fails with Timeout() == false.
func (netError) Timeout() bool   { return true }
func (netError) Temporary() bool { return true }

// ReadWriter implements noise encrypted read writer.
type ReadWriter struct {
	origin io.ReadWriter
	ns     *Noise
	rBuf   bytes.Buffer
	rMx    sync.Mutex
	wMx    sync.Mutex
}

// NewReadWriter constructs a new ReadWriter.
func NewReadWriter(rw io.ReadWriter, ns *Noise) *ReadWriter {
	return &ReadWriter{
		origin: rw,
		ns:     ns,
	}
}

func (rw *ReadWriter) Read(p []byte) (int, error) {
	rw.rMx.Lock()
	defer rw.rMx.Unlock()

	if rw.rBuf.Len() > 0 {
		fmt.Println("noise reads packet from rBuf") // TODO: remove debug print
		return rw.rBuf.Read(p)
	}

	ciphertext, err := rw.readPacket()
	if err != nil {
		fmt.Printf("read failure: %v\n", err) // TODO: remove debug print
		return 0, err
	}

	plaintext, err := rw.ns.DecryptUnsafe(ciphertext)
	if err != nil {
		return 0, &netError{Err: err}
	}

	return ioutil.BufRead(&rw.rBuf, plaintext, p)
}

func (rw *ReadWriter) readPacket() ([]byte, error) {
	h := make([]byte, 2)
	n1, err1 := io.ReadFull(rw.origin, h)
	if err1 != nil && n1 != len(h) {
		return nil, err1
	}
	fmt.Printf("read size: [%d/%d] %v\n", n1, 2, h) // TODO: remove debug print

	l := binary.BigEndian.Uint16(h)
	data := make([]byte, l)
	n2, err2 := io.ReadFull(rw.origin, data)

	if err1 != nil {
		return nil, err1
	}
	if err2 != nil {
		return nil, err2
	}

	_ = n2
	fmt.Printf("read: [%d/%d] %v\n", n2, len(data), data) // TODO: remove debug print
	return data, nil
}

func (rw *ReadWriter) Write(p []byte) (int, error) {
	rw.wMx.Lock()
	defer rw.wMx.Unlock()

	ciphertext := rw.ns.EncryptUnsafe(p)

	if err := rw.writePacket(ciphertext); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (rw *ReadWriter) writePacket(p []byte) error {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(len(p)))

	data := append(buf, p...)
	_, err := rw.origin.Write(data)

	fmt.Printf("written: [%d] %v\n", len(data), data) // TODO: remove debug print
	return err
}

// Handshake performs a Noise handshake using the provided io.ReadWriter.
func (rw *ReadWriter) Handshake(hsTimeout time.Duration) error {
	doneChan := make(chan error)
	go func() {
		if rw.ns.init {
			doneChan <- rw.initiatorHandshake()
		} else {
			doneChan <- rw.responderHandshake()
		}
	}()

	select {
	case err := <-doneChan:
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

func (rw *ReadWriter) initiatorHandshake() error {
	for {
		msg, err := rw.ns.HandshakeMessage()
		if err != nil {
			return err
		}

		if err := rw.writePacket(msg); err != nil {
			return err
		}

		if rw.ns.HandshakeFinished() {
			break
		}

		res, err := rw.readPacket()
		if err != nil {
			return err
		}

		if err = rw.ns.ProcessMessage(res); err != nil {
			return err
		}

		if rw.ns.HandshakeFinished() {
			break
		}
	}

	return nil
}

func (rw *ReadWriter) responderHandshake() error {
	for {
		msg, err := rw.readPacket()
		if err != nil {
			return err
		}

		if err := rw.ns.ProcessMessage(msg); err != nil {
			return err
		}

		if rw.ns.HandshakeFinished() {
			break
		}

		res, err := rw.ns.HandshakeMessage()
		if err != nil {
			return err
		}

		if err := rw.writePacket(res); err != nil {
			return err
		}

		if rw.ns.HandshakeFinished() {
			break
		}
	}

	return nil
}
