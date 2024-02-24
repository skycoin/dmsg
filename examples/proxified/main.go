package main

import (
	"context"

	"net/http"
	"time"

	"github.com/skycoin/skywire-utilities/pkg/logging"
	"github.com/skycoin/skywire-utilities/pkg/skyenv"

	"github.com/skycoin/skywire-utilities/pkg/cipher"
	"golang.org/x/net/proxy"

	"github.com/skycoin/dmsg/pkg/disc"
	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
)

func main() {
	log := logging.MustGetLogger("proxified")

	// generate keys for clients
	respPK, respSK := cipher.GenerateKeyPair()
	initPK, initSK := cipher.GenerateKeyPair()

	// ports to listen by clients. can be any free port
	var initPort, respPort uint16 = 1563, 1563

	// Configure SOCKS5 proxy dialer
	proxyAddr := "127.0.0.1:1080" // use skysocks-client skywire proxy address
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		log.Fatalf("Error creating SOCKS5 dialer: %v", err)
	}

	// Configure custom HTTP transport with SOCKS5 proxy
	transport := &http.Transport{
		Dial: dialer.Dial,
	}

	// Configure HTTP client with custom transport
	httpClient := &http.Client{
		Transport: transport,
	}

	// instantiate clients with custom config
	respC := dmsg.NewClient(respPK, respSK, disc.NewHTTP(skyenv.DmsgDiscAddr, httpClient, log), dmsg.DefaultConfig())
	go respC.Serve(context.Background())

	initC := dmsg.NewClient(initPK, initSK, disc.NewHTTP(skyenv.DmsgDiscAddr, &http.Client{}, log), dmsg.DefaultConfig())
	go initC.Serve(context.Background())

	time.Sleep(2 * time.Second)

	// bind to port and start listening for incoming messages
	initL, err := initC.Listen(initPort)
	if err != nil {
		log.Fatalf("Error listening by initiator on port %d: %v", initPort, err)
	}

	// bind to port and start listening for incoming messages
	respL, err := respC.Listen(respPort)
	if err != nil {
		log.Fatalf("Error listening by responder on port %d: %v", respPort, err)
	}

	initTp, err := initC.DialStream(context.Background(), dmsg.Addr{PK: respPK, Port: respPort})
	if err != nil {
		log.Fatalf("Error dialing responder: %v", err)
	}

	respTp, err := respL.AcceptStream()
	if err != nil {
		log.Fatalf("Error accepting inititator: %v", err)
	}

	payload := "Hello there!"
	_, err = initTp.Write([]byte(payload))
	if err != nil {
		log.Fatalf("Error writing to initiator's stream: %v", err)
	}

	recvBuf := make([]byte, len(payload))
	_, err = respTp.Read(recvBuf)
	if err != nil {
		log.Fatalf("Error reading from responder's stream: %v", err)
	}

	log.Printf("Responder accepted: %s", string(recvBuf))

	payload = "General Kenobi"
	_, err = respTp.Write([]byte(payload))
	if err != nil {
		log.Fatalf("Error writing response: %v", err)
	}

	initRecvBuf := make([]byte, len(payload))
	_, err = initTp.Read(initRecvBuf)
	if err != nil {
		log.Fatalf("Error reading response: %v", err)
	}

	log.Printf("Initiator accepted: %s", string(initRecvBuf))

	if err := initTp.Close(); err != nil {
		log.Fatalf("Error closing initiator's stream: %v", err)
	}

	if err := respTp.Close(); err != nil {
		log.Fatalf("Error closing responder's stream: %v", err)
	}

	if err := initL.Close(); err != nil {
		log.Fatalf("Error closing initiator's listener: %v", err)
	}

	if err := respL.Close(); err != nil {
		log.Fatalf("Error closing responder's listener: %v", err)
	}

	if err := initC.Close(); err != nil {
		log.Fatalf("Error closing initiator: %v", err)
	}

	if err := respC.Close(); err != nil {
		log.Fatalf("Error closing responder: %v", err)
	}
}
