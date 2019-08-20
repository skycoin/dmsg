package main

import (
	"context"
	"log"

	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/disc"
)

func main() {
	// generate keys for clients
	rPK, rSK := cipher.GenerateKeyPair()
	iPK, iSK := cipher.GenerateKeyPair()

	// ports to listen by clients. can be any free port
	var iPort, rPort uint16 = 1563, 1563

	// instantiate discovery
	dc := disc.NewHTTP("https://messaging.discovery.skywire.skycoin.net")

	// instantiate clients
	responder := dmsg.NewClient(rPK, rSK, dc)
	initiator := dmsg.NewClient(iPK, iSK, dc)

	// connect to the DMSG server
	if err := responder.InitiateServerConnections(context.Background(), 1); err != nil {
		log.Fatalf("Error initiating server connections by responder: %v", err)
	}

	// connect to the DMSG server
	if err := initiator.InitiateServerConnections(context.Background(), 1); err != nil {
		log.Fatalf("Error initiating server connections by initiator: %v", err)
	}

	// bind to port and start listening for incoming messages
	iListener, err := initiator.Listen(iPort)
	if err != nil {
		log.Fatalf("Error listening by initiator on port %d: %v", iPort, err)
	}

	// bind to port and start listening for incoming messages
	rListener, err := responder.Listen(rPort)
	if err != nil {
		log.Fatalf("Error listening by responder on port %d: %v", rPort, err)
	}

	// dial responder via DMSG
	iTp, err := initiator.Dial(context.Background(), rPK, rPort)
	if err != nil {
		log.Fatalf("Error dialing responder: %v", err)
	}

	// accept connection. `AcceptTransport` returns an object exposing `transport` features
	// thus, `Accept` could also be used here returning `net.Conn` interface. depends on your needs
	rTp, err := rListener.AcceptTransport()
	if err != nil {
		log.Fatalf("Error accepting inititator: %v", err)
	}

	// initiator writes to his transport
	_, err = iTp.Write([]byte("Hello there!"))
	if err != nil {
		log.Fatalf("Error writing to initiator's transport: %v", err)
	}

	// responder reads from his transport
	recBuf := make([]byte, 12)
	_, err = rTp.Read(recBuf)
	if err != nil {
		log.Fatalf("Error reading from responder's transport: %v", err)
	}

	log.Printf("Responder accepted: %s", string(recBuf))

	// responder writes to his transport
	_, err = rTp.Write([]byte("General Kenobi"))
	if err != nil {
		log.Fatalf("Error writing response: %v", err)
	}

	// initiator reads from his transport
	iRecBuf := make([]byte, 14)
	_, err = iTp.Read(iRecBuf)
	if err != nil {
		log.Fatalf("Error reading response: %v", err)
	}

	log.Printf("Initiator accepted: %s", string(iRecBuf))

	// close transport
	if err := iTp.Close(); err != nil {
		log.Fatalf("Error closing initiator's transport: %v", err)
	}

	// close transport
	if err := rTp.Close(); err != nil {
		log.Fatalf("Error closing responder's transport: %v", err)
	}

	// close listener
	if err := iListener.Close(); err != nil {
		log.Fatalf("Error closing initiator's listener: %v", err)
	}

	// close listener
	if err := rListener.Close(); err != nil {
		log.Fatalf("Error closing responder's listener: %v", err)
	}

	// close client
	if err := initiator.Close(); err != nil {
		log.Fatalf("Error closing initiator: %v", err)
	}

	// close client
	if err := responder.Close(); err != nil {
		log.Fatalf("Error closing responder: %v", err)
	}
}
