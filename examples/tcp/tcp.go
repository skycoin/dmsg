package main

import (
	"log"
	"net"
	"os"
)

func main() {
	// Start a TCP server listening on port 8000
	listener, err := net.Listen("tcp", os.Args[1]) //":8000")
	if err != nil {
		log.Fatal("Failed to start server:", err)
		return
	}
	defer listener.Close()
	log.Println("TCP server started on port", os.Args[1])

	// Accept and handle incoming connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Failed to accept connection:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Println("Handling Connection")
	// Send a greeting message to the client
	message := "Hello, World!\n"
	_, err := conn.Write([]byte(message))
	if err != nil {
		log.Println("Error writing response:", err)
		return
	}
}
