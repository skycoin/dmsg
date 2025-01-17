// Package cli internal/cli/cli.go
package cli

import (
	"context"
	"fmt"
	"net/http"

	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"

	"github.com/skycoin/dmsg/pkg/disc"
	"github.com/skycoin/dmsg/pkg/dmsg"
)

// StartDmsg starts dmsg returns a dmsg client for the given dmsg discovery
func StartDmsg(ctx context.Context, dmsgLogger *logging.Logger, pk cipher.PubKey, sk cipher.SecKey, httpClient *http.Client, dmsgDisc string, dmsgSessions int) (dmsgC *dmsg.Client, stop func(), err error) {
	dmsgC = dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, httpClient, dmsgLogger), &dmsg.Config{MinSessions: dmsgSessions})
	go dmsgC.Serve(context.Background())

	stop = func() {
		err := dmsgC.Close()
		dmsgLogger.WithError(err).Debug("Disconnected from dmsg network.")
		fmt.Printf("\n")
	}
	dmsgLogger.WithField("public_key", pk.String()).WithField("dmsg_disc", dmsgDisc).
		Debug("Connecting to dmsg network...")
	select {
	case <-ctx.Done():
		stop()
		return nil, nil, ctx.Err()

	case <-dmsgC.Ready():
		dmsgLogger.Debug("Dmsg network ready.")
		return dmsgC, stop, nil
	}
}

/*
// StartDmsg starts dmsg returns a dmsg client for the given discovery or discoveries
func StartMultipleDmsg(dmsgLogger *logging.Logger, ctx []context.Context, pk cipher.PubKey, sk cipher.SecKey, httpClient []*http.Client, dmsgDiscs []string, dmsgSessions []int) (dmsgC []*dmsg.Client, stop []func(), errs []error) {

	lengths := map[string]int{
		"ctx":        len(ctx),
		"dmsgDiscs":  len(dmsgDiscs),
		"dmsgSessions": len(dmsgSessions),
		"httpClient": len(httpClient),
	}

	var referenceLen int
	for key, length := range lengths {
		if referenceLen == 0 {
			referenceLen = length
		} else if length != referenceLen {
			errs = append(errs, fmt.Errorf("all arrays input to StartDmsg must be the same length. Mismatch detected:\n%v", lengths))
			return nil, nil, errs
		}
	}

	// Validate `dmsgDiscs` for duplicates and empty strings
	seen := make(map[string]bool)
	for _, disc := range dmsgDiscs {
		if disc == "" {
			errs = append(errs, fmt.Errorf("dmsgDiscs contains an empty string"))
			continue
		}
		if seen[disc] {
			errs = append(errs, fmt.Errorf("dmsgDiscs contains duplicate entry: %q", disc))
		}
		seen[disc] = true
	}

	// Return errors if validation fails
	if len(errs) > 0 {
		return nil, nil, errs
	}


  for i, _ := range dmsgDisc {
	dmsgC = append(dmsgc, dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc[i], httpClient, dmsgcurlLog), &dmsg.Config{MinSessions: dmsgSessions[i]}))
	go dmsgC[i].Serve(ctx[i])

	stop = append(stop, func() {
		err := dmsgC.Close()
		dmsgLogger.WithError(err).Debug("Disconnected from dmsg network.")
		fmt.Printf("\n")
	}
	dmsgLogger.WithField("public_key", pk.String()).WithField("dmsg_disc", dmsgDisc[i]).
		Debug("Connecting to dmsg network...")

	select {
	case <-ctx[i].Done():
		stop[i]()
    errs = append(errs,ctx[i].Err())
		return nil, nil, errs

	case <-dmsgC[i].Ready():
		dmsgLogger.Debug("Dmsg network ready for client: %d",i)
	}
  return dmsgC, stop, nil
}
}
*/
