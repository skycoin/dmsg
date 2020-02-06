package dmsgpty

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"net/url"
	"sync/atomic"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/cipher"
)

// Host represents the main instance of dmsgpty.
type Host struct {
	dmsgC *dmsg.Client
	wl    Whitelist
	mux   hostMux

	cliN uint32
}

// NewHost creates a new dmsgpty.Host with a given dmsg.Client and whitelist.
func NewHost(dmsgC *dmsg.Client, wl Whitelist) *Host {
	host := new(Host)
	host.dmsgC = dmsgC
	host.wl = wl
	host.prepareMux()
	return host
}

// prepareMux prepares the endpoints of the host.
// These endpoints can be accessed via CLI or dmsg (if the remote entity is whitelisted).
func (h *Host) prepareMux() {

	h.mux.Handle(WhitelistURI, func(ctx context.Context, uri *url.URL, rpcS *rpc.Server) error {
		return rpcS.RegisterName(WhitelistRPCName, NewWhitelistGateway(h.wl))
	})

	h.mux.Handle(PtyURI, func(ctx context.Context, uri *url.URL, rpcS *rpc.Server) error {
		pty := NewPty()
		go func() {
			<-ctx.Done()
			h.log().
				WithError(pty.Stop()).
				Debug("PTY stopped.")
		}()
		return rpcS.RegisterName(PtyRPCName, NewPtyGateway(pty))
	})

	h.mux.Handle(PtyProxyURI, func(ctx context.Context, uri *url.URL, rpcS *rpc.Server) error {
		q := uri.Query()

		// Get query values.
		var pk cipher.PubKey
		if err := pk.Set(q.Get("pk")); err != nil {
			return fmt.Errorf("invalid query value 'pk': %v", err)
		}
		var port uint16
		if _, err := fmt.Sscan(q.Get("port"), &port); err != nil {
			return fmt.Errorf("invalid query value 'port': %v", err)
		}

		// Proxy request.
		stream, err := h.dmsgC.DialStream(ctx, dmsg.Addr{PK: pk, Port: port})
		if err != nil {
			return err
		}
		go func() {
			<-ctx.Done()
			h.log().
				WithError(stream.Close()).
				Debug("Closed proxy dmsg stream.")
		}()

		ptyC, err := NewPtyClient(stream)
		if err != nil {
			return err
		}
		go func() {
			<-ctx.Done()
			h.log().
				WithError(ptyC.Close()).
				Debug("Closed proxy pty client.")
		}()
		return rpcS.RegisterName(PtyRPCName, NewProxyGateway(ptyC))
	})
}

// ServeCLI listens for CLI connections via the provided listener.
func (h *Host) ServeCLI(ctx context.Context, lis net.Listener) error {
	log := logging.MustGetLogger("dmsgpty:cli-server")

	for {
		log := log.WithField("cli_id", atomic.AddUint32(&h.cliN, 1))

		conn, err := lis.Accept()
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Temporary() {
				log.Warn("Failed to accept CLI connection with temporary error, continuing...")
				continue
			}
			if err == io.ErrClosedPipe {
				log.Info("Cleanly stopped serving.")
				return nil
			}
			log.Error("Failed to accept CLI connection with permanent error.")
			return err
		}

		log.Info("CLI connection accepted.")
		go h.serveConn(ctx, log, conn)
	}
}

// ListenAndServe serves the host over the dmsg network via the given dmsg port.
func (h *Host) ListenAndServe(ctx context.Context, port uint16) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lis, err := h.dmsgC.Listen(port)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		h.log().
			WithError(lis.Close()).
			Info("Serve() ended.")
	}()

	for {
		stream, err := lis.AcceptStream()
		if err != nil {
			log := h.log().WithError(err)
			if err, ok := err.(net.Error); ok && err.Temporary() {
				log.Warn("Failed to accept dmsg.Stream with temporary error, continuing...")
				continue
			}
			if err == io.ErrClosedPipe {
				log.Info("Cleanly stopped serving.")
				return nil
			}
			log.Error("Failed to accept dmsg.Stream with permanent error.")
			return err
		}

		rPK := stream.RawRemoteAddr().PK
		log := h.log().WithField("remote_pk", rPK.String())
		log.Info("Processing dmsg.Stream...")

		if !h.authorize(log, rPK) {
			log.Warn("dmsg.Stream rejected.")
			if err := stream.Close(); err != nil {
				log.WithError(err).Warn("Stream closed with error.")
			}
			continue
		}

		log.Info("dmsg.Stream accepted.")
		log = stream.Logger().WithField("dmsgpty", "stream")
		go h.serveConn(ctx, log, stream)
	}
}

// serveConn serves a CLI connection or dmsg stream.
func (h *Host) serveConn(ctx context.Context, log logrus.FieldLogger, conn net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		log.WithError(conn.Close()).
			Info("serveConn() closed the connection.")
	}()

	log.WithError(h.mux.ServeConn(ctx, conn)).
		Info("serveConn() stopped serving.")
}

// authorize returns true if the provided public key is whitelisted.
func (h *Host) authorize(log logrus.FieldLogger, rPK cipher.PubKey) bool {
	ok, err := h.wl.Get(rPK)
	if err != nil {
		log.WithError(err).Panic("dmsgpty.Whitelist error.")
		return false
	}
	if !ok {
		log.Warn("Public key rejected by whitelist.")
		return false
	}
	return true
}

// log returns the logrus.FieldLogger that should be used for all log outputs.
func (h *Host) log() logrus.FieldLogger {
	return h.dmsgC.Logger().WithField("dmsgpty", "host")
}
