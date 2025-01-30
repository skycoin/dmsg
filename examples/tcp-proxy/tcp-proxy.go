package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("requires two arguments; usage: tcp-proxy <target-port> <source-port>")
	}
	sourcePort, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatalf("Failed to parse tcp source port string \"%v\" to int: %v", sourcePort, err)
	}
	targetPort, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to parse tcp target port string \"%v\" to int: %v", targetPort, err)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", sourcePort))
	if err != nil {
		log.Fatalf("Failed to start TCP listener on port %d: %v", sourcePort, err)
	}
	defer listener.Close()
	log.Printf("TCP proxy started: Listening on port %d and forwarding to port %d", sourcePort, targetPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(conn, targetPort)
	}
}

func handleConnection(conn net.Conn, targetPort int) {
	defer conn.Close()

	targetAddr := fmt.Sprintf("localhost:%d", targetPort)
	target, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Failed to dial target server %s: %v", targetAddr, err)
		return
	}
	defer target.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		_, err := io.Copy(target, conn)
		if err != nil && !isClosedConnErr(err) {
			log.Printf("Error copying from client to target: %v", err)
		}
		target.Close()
		wg.Done()
	}()

	go func() {
		_, err := io.Copy(conn, target)
		if err != nil && !isClosedConnErr(err) {
			log.Printf("Error copying from target to client: %v", err)
		}
		conn.Close()
		wg.Done()
	}()

	wg.Wait()
}

func isClosedConnErr(err error) bool {
	if err == io.EOF {
		return true
	}
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
