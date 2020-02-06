package dmsgpty

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"net/url"
)

// Constants related to pty.
const (
	PtyRPCName  = "pty"
	PtyURI      = "dmsgpty/pty"
	PtyProxyURI = "dmsgpty/proxy"
)

// Constants related to whitelist.
const (
	WhitelistRPCName = "whitelist"
	WhitelistURI     = "dmsgpty/whitelist"
)

// empty is used for RPC calls.
var empty struct{}

func processRPCError(err error) error {
	if err != nil {
		switch err.Error() {
		case io.EOF.Error():
			return io.EOF
		case io.ErrUnexpectedEOF.Error():
			return io.ErrUnexpectedEOF
		case io.ErrClosedPipe.Error():
			return io.ErrClosedPipe
		case io.ErrNoProgress.Error():
			return io.ErrNoProgress
		case io.ErrShortBuffer.Error():
			return io.ErrShortBuffer
		case io.ErrShortWrite.Error():
			return io.ErrShortWrite
		default:
			return err
		}
	}
	return nil
}

// readRequest reads the request.
// Each request must be smaller than 255 bytes.
func readRequest(r io.Reader) (*url.URL, error) {
	prefix := make([]byte, 1)
	if _, err := io.ReadFull(r, prefix); err != nil {
		return nil, fmt.Errorf("readRequest: failed to read prefix: %v", err)
	}

	rawURI := make([]byte, prefix[0])
	if _, err := io.ReadFull(r, rawURI); err != nil {
		return nil, fmt.Errorf("readRequest: failed to read URI: %v", err)
	}
	rawURI = bytes.TrimPrefix(rawURI, []byte{'/'})

	uri, err := url.Parse(string(rawURI))
	if err != nil {
		return nil, fmt.Errorf("readRequest: failed to parse URI: %v", err)
	}
	return uri, nil
}

func writeRequest(w io.Writer, uri string) error {
	l := len(uri)
	if l > math.MaxUint8 {
		return fmt.Errorf("request URI cannot be larger than %d bytes", math.MaxUint8)
	}
	bufW := bufio.NewWriter(w)
	if err := bufW.WriteByte(byte(l)); err != nil {
		return err
	}
	if _, err := bufW.WriteString(uri); err != nil {
		return err
	}
	return bufW.Flush()
}
