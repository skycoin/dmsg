package dmsgpty

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/rpc"
	"net/url"
	"path"
	"strings"
)

type muxEntry struct {
	pat string
	fn  handleFunc
}

type hostMux struct {
	entries []muxEntry
}

type handleFunc func(ctx context.Context, uri *url.URL, rpcS *rpc.Server) error

func (h *hostMux) Handle(pattern string, fn handleFunc) {
	pattern = strings.TrimPrefix(pattern, "/")
	if _, err := path.Match(pattern, ""); err != nil {
		panic(err)
	}
	h.entries = append(h.entries, muxEntry{
		pat: pattern,
		fn:  fn,
	})
}

func (h *hostMux) ServeConn(ctx context.Context, conn net.Conn) error {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	uri, err := readRequest(conn)
	if err != nil {
		return err
	}
	for _, entry := range h.entries {
		ok, err := path.Match(entry.pat, uri.EscapedPath())
		if err != nil {
			panic(err)
		}
		if !ok {
			continue
		}
		rpcS := rpc.NewServer()
		if err := entry.fn(ctx, uri, rpcS); err != nil {
			return err
		}
		rpcS.ServeConn(conn)
		return nil
	}
	return errors.New("invalid request")
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
