package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	// Start a TCP server listening on port 8000
	listener, err := net.Listen("tcp", os.Args[1]) //":8000")
	if err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		return
	}
	defer listener.Close()
	fmt.Println("TCP server started on port " + os.Args[1])

	// Accept and handle incoming connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Send a greeting message to the client
	message := "Hello, World!\n"
	_, err := conn.Write([]byte(message))
	if err != nil {
		fmt.Printf("Error writing response: %v\n", err)
		return
	}
}
