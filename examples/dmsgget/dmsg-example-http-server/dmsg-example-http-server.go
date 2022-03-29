package main

import (
	"context"
	"flag"
	"net/http"

	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/dmsg/pkg/disc"
	dmsg "github.com/skycoin/dmsg/pkg/dmsg"

	"github.com/skycoin/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire-utilities/pkg/cmdutil"
)

var (
	dir      = "." // local dir to serve via http
	dmsgDisc = "http://dmsgd.skywire.skycoin.com"
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

	c := dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, &http.Client{}, log), dmsg.DefaultConfig())
	defer func() {
		if err := c.Close(); err != nil {
			log.WithError(err).Error()
		}
	}()

	go c.Serve(context.Background())

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
