package main

import (
	"context"
	"flag"
	"os"

	"github.com/SkycoinProject/skycoin/src/util/logging"

	"github.com/SkycoinProject/dmsg/cmdutil"
	"github.com/SkycoinProject/dmsg/dmsgget"
)

var (
	log   = logging.MustGetLogger(dmsgget.ExecName)
	skStr = os.Getenv("DMSGGET_SK")
	dg    = dmsgget.New(flag.CommandLine)
)

func main() {
	flag.Parse()

	ctx, cancel := cmdutil.SignalContext(context.Background(), log)
	defer cancel()

	if err := dg.Run(ctx, log, skStr, flag.Args()); err != nil {
		log.WithError(err).Fatal()
	}
}
