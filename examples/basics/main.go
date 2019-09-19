package main

import (
	"context"
	"log"

	"golang.org/x/net/nettest"

	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
)

func main() {
	// generate keys for clients
	respPK, respSK := cipher.GenerateKeyPair()
	initPK, initSK := cipher.GenerateKeyPair()

	// ports to listen by clients. can be any free port
	var initPort, respPort uint16 = 1563, 1563

	// instantiate discovery
	// dc := disc.NewHTTP("https://messaging.discovery.skywire.skycoin.net")
	dc := disc.NewMock()

	// instantiate server
	srv, err := createDmsgSrv(dc)
	if err != nil {
		log.Fatalf("Error initiating server: %v", err)
	}
	defer func() { _ = srv.Close() }() //nolint:errcheck

	// instantiate clients
	respC := dmsg.NewClient(respPK, respSK, dc)
	initC := dmsg.NewClient(initPK, initSK, dc)

	// connect to the DMSG server
	if err := respC.InitiateServerConnections(context.Background(), 1); err != nil {
		log.Fatalf("Error initiating server connections by responder: %v", err)
	}

	// connect to the DMSG server
	if err := initC.InitiateServerConnections(context.Background(), 1); err != nil {
		log.Fatalf("Error initiating server connections by initiator: %v", err)
	}

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

	// dial responder via DMSG
	initTp, err := initC.Dial(context.Background(), respPK, respPort)
	if err != nil {
		log.Fatalf("Error dialing responder: %v", err)
	}

	// Accept connection. `AcceptTransport` returns an object exposing `transport` features
	// thus, `Accept` could also be used here returning `net.Conn` interface. depends on your needs
	respTp, err := respL.AcceptTransport()
	if err != nil {
		log.Fatalf("Error accepting inititator: %v", err)
	}

	// initiator writes to it's transport
	payload := "Hello there!"
	_, err = initTp.Write([]byte(payload))
	if err != nil {
		log.Fatalf("Error writing to initiator's transport: %v", err)
	}

	// responder reads from it's transport
	recvBuf := make([]byte, len(payload))
	_, err = respTp.Read(recvBuf)
	if err != nil {
		log.Fatalf("Error reading from responder's transport: %v", err)
	}

	log.Printf("Responder accepted: %s", string(recvBuf))

	// responder writes to it's transport
	payload = "General Kenobi"
	_, err = respTp.Write([]byte(payload))
	if err != nil {
		log.Fatalf("Error writing response: %v", err)
	}

	// initiator reads from it's transport
	initRecvBuf := make([]byte, len(payload))
	_, err = initTp.Read(initRecvBuf)
	if err != nil {
		log.Fatalf("Error reading response: %v", err)
	}

	log.Printf("Initiator accepted: %s", string(initRecvBuf))

	// close transport
	if err := initTp.Close(); err != nil {
		log.Fatalf("Error closing initiator's transport: %v", err)
	}

	// close transport
	if err := respTp.Close(); err != nil {
		log.Fatalf("Error closing responder's transport: %v", err)
	}

	// close listener
	if err := initL.Close(); err != nil {
		log.Fatalf("Error closing initiator's listener: %v", err)
	}

	// close listener
	if err := respL.Close(); err != nil {
		log.Fatalf("Error closing responder's listener: %v", err)
	}

	// close client
	if err := initC.Close(); err != nil {
		log.Fatalf("Error closing initiator: %v", err)
	}

	// close client
	if err := respC.Close(); err != nil {
		log.Fatalf("Error closing responder: %v", err)
	}
}

func createDmsgSrv(dc disc.APIClient) (*dmsg.Server, error) {
	pk, sk, err := cipher.GenerateDeterministicKeyPair([]byte("s"))
	if err != nil {
		return nil, err
	}
	l, err := nettest.NewLocalListener("tcp")
	if err != nil {
		return nil, err
	}
	srv, err := dmsg.NewServer(pk, sk, "", l, dc)
	if err != nil {
		return nil, err
	}
	go func() { _ = srv.Serve() }() //nolint:errcheck
	return srv, nil
}
