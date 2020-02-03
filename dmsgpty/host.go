package dmsgpty

import (
	"context"
	"fmt"
	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"net/rpc"
	"net/url"
	"sync/atomic"
)

type Host struct {
	dmsgC *dmsg.Client
	wl    Whitelist
	mux   hostMux

	cliN uint32
}

func NewHost(dmsgC *dmsg.Client, wl Whitelist) *Host {
	host := new(Host)
	host.dmsgC = dmsgC
	host.wl = wl
	host.prepareMux()
	return host
}

func (h *Host) prepareMux() {
	h.mux.Handle("dmsgpty/pty",
		func(ctx context.Context, uri *url.URL, rpcS *rpc.Server) error {
			pty := NewPty()
			go func() {
				<-ctx.Done()
				_ = pty.Stop()
			}()
			return rpcS.RegisterName(PtyRPCName, NewPtyGateway(pty))
		})

	h.mux.Handle("dmsgpty/whitelist",
		func(ctx context.Context, uri *url.URL, rpcS *rpc.Server) error {
			return rpcS.RegisterName(WhitelistRPCName, NewWhitelistGateway(h.wl))
		})

	h.mux.Handle("dmsgpty/proxy",
		func(ctx context.Context, uri *url.URL, rpcS *rpc.Server) error {
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
			if err := writeRequest(stream, "dmsgpty/pty"); err != nil {
				return err
			}

			//log := stream.Logger().WithField("dmsgpty", "proxied_stream")
			ptyC := NewPtyClient(stream)
			return rpcS.RegisterName(PtyRPCName, NewProxyGateway(ptyC))
		})
}

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

func (h *Host) log() logrus.FieldLogger {
	return h.dmsgC.Logger().WithField("dmsgpty", "host")
}