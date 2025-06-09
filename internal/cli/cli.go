// Package cli internal/cli/cli.go
package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"

	"github.com/skycoin/dmsg/pkg/direct"
	"github.com/skycoin/dmsg/pkg/disc"
	"github.com/skycoin/dmsg/pkg/dmsg"
	"github.com/skycoin/dmsg/pkg/dmsghttp"
)

// StartDmsg starts dmsg returns a dmsg client for the given dmsg discovery
func StartDmsg(ctx context.Context, dmsgLogger *logging.Logger, pk cipher.PubKey, sk cipher.SecKey, httpClient *http.Client, dmsgDisc string, dmsgSessions int) (dmsgC *dmsg.Client, stop func(), err error) {
	if dmsgLogger == nil {
		return nil, nil, fmt.Errorf("nil logger")
	}

	dmsgC = dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, httpClient, dmsgLogger), &dmsg.Config{MinSessions: dmsgSessions})
	dmsgLogger.Debug("Created dmsg client.")

	go dmsgC.Serve(context.Background())
	dmsgLogger.Debug("dmsgclient.Serve(context.Background())")

	stop = func() {
		err := dmsgC.Close()
		dmsgLogger.WithError(err).Debug("Disconnected from dmsg network.\n")
		log.Println()
	}
	dmsgLogger.WithField("dmsg_disc", dmsgDisc).Debug("Connecting to dmsg network...\n")
	dmsgLogger.WithField("client public_key", pk.String()).Debug("\n")
	select {
	case <-ctx.Done():
		stop()
		return nil, nil, ctx.Err()

	case <-dmsgC.Ready():
		dmsgLogger.Debug("Dmsg network ready.")
		return dmsgC, stop, nil
	}
}

// StartDmsgDirect starts dmsg returns a dmsg direct client
func StartDmsgDirect(ctx context.Context, dmsgLogger *logging.Logger, pk cipher.PubKey, sk cipher.SecKey, httpClient *http.Client, dmsgDiscAddr string, dmsgSessions int, destination string) (dmsgC *dmsg.Client, stop func(), err error) {
	servers := make([]*disc.Entry, len(dmsg.Prod.DmsgServers))
	for i := range dmsg.Prod.DmsgServers {
		servers[i] = &dmsg.Prod.DmsgServers[i]
	}
	if len(servers) == 0 {
		return nil, nil, fmt.Errorf("no DMSG servers configured")
	}

	destinationPk := cipher.PubKey{}
	if err = destinationPk.UnmarshalText([]byte(destination)); err != nil {
		return nil, nil, fmt.Errorf("invalid destination public key: %w", err)
	}

	var lastErr error
	for _, srv := range servers {
		dClient := direct.NewClient([]*disc.Entry{srv}, dmsgLogger)

		clientEntry := &disc.Entry{
			Client: &disc.Client{
				DelegatedServers: []cipher.PubKey{srv.Static},
			},
			Static: destinationPk,
		}

		if err := dClient.PostEntry(ctx, clientEntry); err != nil {
			dmsgLogger.WithError(err).Warnf("failed to post entry to DMSG server %s", srv.Static)
			lastErr = err
			continue
		}

		dmsgConfig := dmsg.DefaultConfig()
		dmsgConfig.MinSessions = dmsgSessions

		dmsgC, stop, err = direct.StartDmsg(ctx, dmsgLogger, pk, sk, dClient, dmsgConfig)
		if err != nil {
			dmsgLogger.WithError(err).Warnf("failed to start DMSG client with server %s", srv.Static)
			lastErr = err
			continue
		}

		dmsgHTTP := &http.Client{Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgC)}
		resp, err := dmsgHTTP.Get(dmsgDiscAddr + "/health")
		if err != nil {
			dmsgLogger.WithError(err).Warnf("failed to reach discovery server via DMSG using server %s", srv.Static)
			stop() // Clean up
			lastErr = err
			continue
		}
		resp.Body.Close()

		// Success!
		return dmsgC, stop, nil
	}

	return nil, nil, fmt.Errorf("could not connect to dmsg discovery server through any DMSG server: last error: %w", lastErr)
}

// StartDmsgDirectWithServers starts a DMSG client using the provided set of DMSG servers.
// It attempts to connect and validate discovery access via the full server set.
func StartDmsgDirectWithServers(ctx context.Context, dmsgLogger *logging.Logger, pk cipher.PubKey, sk cipher.SecKey, httpClient *http.Client, dmsgDiscAddr string, dmsgServers []*disc.Entry, dmsgSessions int, destination string) (dmsgC *dmsg.Client, stop func(), err error) {

	if len(dmsgServers) == 0 {
		return nil, nil, fmt.Errorf("no DMSG servers provided")
	}

	destinationPk := cipher.PubKey{}
	if err = destinationPk.UnmarshalText([]byte(destination)); err != nil {
		return nil, nil, fmt.Errorf("invalid destination public key: %w", err)
	}

	// Build direct client with all provided servers
	var keys cipher.PubKeys
	keys = append(keys, pk)
	entries := direct.GetAllEntries(keys, dmsgServers)
	dClient := direct.NewClient(entries, dmsgLogger)

	// Post client entry with all delegated servers
	var delegatedServers []cipher.PubKey
	for _, srv := range dmsgServers {
		delegatedServers = append(delegatedServers, srv.Static)
	}
	clientEntry := &disc.Entry{
		Client: &disc.Client{
			DelegatedServers: delegatedServers,
		},
		Static: destinationPk,
	}
	if err := dClient.PostEntry(ctx, clientEntry); err != nil {
		return nil, nil, fmt.Errorf("failed to post client entry: %w", err)
	}

	// Configure and start DMSG client
	dmsgConfig := dmsg.DefaultConfig()
	dmsgConfig.MinSessions = dmsgSessions

	dmsgC, stop, err = direct.StartDmsg(ctx, dmsgLogger, pk, sk, dClient, dmsgConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start DMSG client: %w", err)
	}

	// Validate that we can access discovery over DMSG
	dmsgHTTP := &http.Client{Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgC)}
	resp, err := dmsgHTTP.Get(dmsgDiscAddr + "/health")
	if err != nil {
		stop() // Cleanup if validation fails
		return nil, nil, fmt.Errorf("failed to reach discovery server via DMSG: %w", err)
	}
	resp.Body.Close()

	return dmsgC, stop, nil
}
