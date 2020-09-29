package main

import (
	"context"
	"flag"
	"os"

	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/dmsg/cmdutil"
	"github.com/skycoin/dmsg/dmsgget"
)

func main() {
	log := logging.MustGetLogger(dmsgget.ExecName)

	skStr := os.Getenv("DMSGGET_SK")

	dg := dmsgget.New(flag.CommandLine)
	flag.Parse()

	ctx, cancel := cmdutil.SignalContext(context.Background(), log)
	defer cancel()

	if err := dg.Run(ctx, log, skStr, flag.Args()); err != nil {
		log.WithError(err).Fatal()
	}
}
