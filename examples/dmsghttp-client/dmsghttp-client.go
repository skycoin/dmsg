package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"

	"github.com/skycoin/dmsg/pkg/dmsg"
	"github.com/skycoin/dmsg/pkg/dmsgclient"
	"github.com/skycoin/dmsg/pkg/dmsghttp"
)

func main() {
	dLog := logging.MustGetLogger("dmsghttp-client")
	dmsgDisc := dmsg.DiscAddr(false)
	parsedURL, err := url.Parse(os.Args[1])
	if err != nil {
		dLog.Fatalf("Failed to parse URL: %v", err)
	}
	pk, sk := cipher.GenerateKeyPair()
	ctx := context.Background()
	dmsgClient, closeDmsg, err := dmsgclient.StartDmsg(ctx, dLog, pk, sk, &http.Client{}, dmsgDisc, 1)
	if err != nil {
		dLog.Fatalf("Failed to start DMSG client: %v", err)
	}
	dLog.Println("started dmsg client")
	defer closeDmsg()
	if dmsgClient == nil {
		dLog.Fatal("DMSG client initialization failed. Exiting.")
	}
	httpClient := &http.Client{
		Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgClient),
	}
	resp, err := httpClient.Get(parsedURL.String())
	if err != nil {
		dLog.Fatalf("Failed to perform GET request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		dLog.Fatalf("Failed to read response body: %v", err)
	}
	fmt.Println(string(body))
}
