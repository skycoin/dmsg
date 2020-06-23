package main

import (
	"context"
	"log"
	"time"

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
	// dc := disc.NewHTTP("http://127.0.0.1:9090")
	dc := disc.NewMock(0)
	maxSessions := 10

	// instantiate server
	sPK, sSK := cipher.GenerateKeyPair()
	srvConf := dmsg.ServerConfig{
		MaxSessions:    maxSessions,
		UpdateInterval: 0,
	}
	srv := dmsg.NewServer(sPK, sSK, dc, &srvConf, nil)

	lis, err := nettest.NewLocalListener("tcp")
	if err != nil {
		panic(err)
	}
	go func() { _ = srv.Serve(lis, "") }() //nolint:errcheck
	time.Sleep(time.Second)

	// instantiate clients
	respC := dmsg.NewClient(respPK, respSK, dc, nil)
	go respC.Serve()

	initC := dmsg.NewClient(initPK, initSK, dc, nil)
	go initC.Serve()

	time.Sleep(time.Second)

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
	initTp, err := initC.DialStream(context.Background(), dmsg.Addr{PK: respPK, Port: respPort})
	if err != nil {
		log.Fatalf("Error dialing responder: %v", err)
	}

	// Accept connection. `AcceptStream` returns an object exposing `stream` features
	// thus, `Accept` could also be used here returning `net.Conn` interface. depends on your needs
	respTp, err := respL.AcceptStream()
	if err != nil {
		log.Fatalf("Error accepting inititator: %v", err)
	}

	// initiator writes to it's stream
	payload := "Hello there!"
	_, err = initTp.Write([]byte(payload))
	if err != nil {
		log.Fatalf("Error writing to initiator's stream: %v", err)
	}

	// responder reads from it's stream
	recvBuf := make([]byte, len(payload))
	_, err = respTp.Read(recvBuf)
	if err != nil {
		log.Fatalf("Error reading from responder's stream: %v", err)
	}

	log.Printf("Responder accepted: %s", string(recvBuf))

	// responder writes to it's stream
	payload = "General Kenobi"
	_, err = respTp.Write([]byte(payload))
	if err != nil {
		log.Fatalf("Error writing response: %v", err)
	}

	// initiator reads from it's stream
	initRecvBuf := make([]byte, len(payload))
	_, err = initTp.Read(initRecvBuf)
	if err != nil {
		log.Fatalf("Error reading response: %v", err)
	}

	log.Printf("Initiator accepted: %s", string(initRecvBuf))

	// close stream
	if err := initTp.Close(); err != nil {
		log.Fatalf("Error closing initiator's stream: %v", err)
	}

	// close stream
	if err := respTp.Close(); err != nil {
		log.Fatalf("Error closing responder's stream: %v", err)
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
