package main

import (
	"context"
	"flag"
	"net/http"

	"github.com/SkycoinProject/skycoin/src/util/logging"

	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/cmdutil"
	"github.com/SkycoinProject/dmsg/disc"
)

var (
	dir      = "." // local dir to serve via http
	dmsgDisc = "http://dmsg.discovery.skywire.skycoin.com"
	dmsgPort = uint(80)
	pk, sk   = cipher.GenerateKeyPair()
)

func init() {
	flag.StringVar(&dir, "dir", dir, "local dir to serve via http")
	flag.StringVar(&dmsgDisc, "disc", dmsgDisc, "dmsg discovery address")
	flag.UintVar(&dmsgPort, "port", dmsgPort, "dmsg port to serve from")
	flag.Var(&sk, "sk", "dmsg secret key")
}

func parse() (err error) {
	flag.Parse()

	pk, err = sk.PubKey()
	return err
}

func main() {
	log := logging.MustGetLogger("dmsg-example-http-server")

	ctx, cancel := cmdutil.SignalContext(context.Background(), log)
	defer cancel()

	if err := parse(); err != nil {
		log.WithError(err).Fatal("Invalid CLI args.")
	}

	c := dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc), dmsg.DefaultConfig())
	defer func() {
		if err := c.Close(); err != nil {
			log.WithError(err).Error()
		}
	}()

	go c.Serve()

	select {
	case <-ctx.Done():
		log.WithError(ctx.Err()).Warn()
		return

	case <-c.Ready():
	}

	lis, err := c.Listen(uint16(dmsgPort))
	if err != nil {
		log.WithError(err).Fatal()
	}
	go func() {
		<-ctx.Done()
		if err := lis.Close(); err != nil {
			log.WithError(err).Error()
		}
	}()

	log.WithField("dir", dir).
		WithField("dmsg_addr", lis.Addr().String()).
		Info("Serving...")

	log.Fatal(http.Serve(lis, http.FileServer(http.Dir(dir))))
}
