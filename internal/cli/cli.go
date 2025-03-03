// Package cli internal/cli/cli.go
package cli

import (
	"context"
	"fmt"
	"net/http"

	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"
	"math/rand/v2"

	"github.com/skycoin/dmsg/pkg/direct"
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

//TODO

func StartDmsgDirect(ctx context.Context, dmsgLogger *logging.Logger, pk cipher.PubKey, sk cipher.SecKey, httpClient *http.Client, _ string, dmsgSessions int) (dmsgC *dmsg.Client, stop func(), err error) { //nolint:all
	var servers []*disc.Entry
	for i := range dmsg.Prod.DmsgServers {
		servers = append(servers, &dmsg.Prod.DmsgServers[i])
	}
	if len(servers) == 0 {
		return nil, nil, fmt.Errorf("no dmsg servers configured")
	}

	var keys cipher.PubKeys
	var delegatedServers []cipher.PubKey

	keys = append(keys, pk)
	entries := direct.GetAllEntries(keys, servers)
	dClient := direct.NewClient(entries, dmsgLogger)
	ctx, cancel := cmdutil.SignalContext(context.Background(), dmsgLogger)
	defer cancel()

	dmsgDC, closeDmsgDC, err := direct.StartDmsg(ctx, dmsgLogger, pk, sk, dClient, dmsg.DefaultConfig())
	if err != nil {
		dmsgLogger.WithError(err).Fatal("failed to start dmsg")
	}
	go dmsgDC.Serve(context.Background())

	servers, err = dClient.AvailableServers(ctx)
	if err != nil {
		dmsgLogger.WithError(err).Fatal("error getting AvailableServers")
	}
	// randomize dmsg servers list
	rand.Shuffle(len(servers), func(i, j int) {
		servers[i], servers[j] = servers[j], servers[i]
	})
	for _, server := range servers {
		delegatedServers = append(delegatedServers, server.Static)
	}

	clientEntry := &disc.Entry{
		Client: &disc.Client{
			DelegatedServers: delegatedServers,
		},
		Static: pk,
	}

	err = dClient.PostEntry(ctx, clientEntry)
	if err != nil {
		dmsgLogger.WithError(err).Fatal("error saving clientEntry")
	}

	stop = func() {
		err := dmsgDC.Close()
		dmsgLogger.WithError(err).Debug("Disconnected from dmsg network.")
		closeDmsgDC()
		fmt.Printf("\n")
	}
	// it technically may not be using the dmsg discovery defined in dmsg.Prod.DmsgDiscovery
	dmsgLogger.WithField("public_key", pk.String()).WithField("dmsg_disc", dmsg.Prod.DmsgDiscovery).Debug("Connecting to dmsg network...")
	// it just uses whatever dmsg discovery that the dmsg server is connected to, by default
	select {
	case <-ctx.Done():
		stop()
		return nil, nil, ctx.Err()

	case <-dmsgDC.Ready():
		dmsgLogger.Debug("Dmsg network ready.")
		return dmsgDC, stop, nil
	}
}
