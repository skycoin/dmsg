// example hello world HTTP
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
)

func main() {
	// Define the HTTP handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		fmt.Fprintf(w, "Hello, World!\n")
	})

	// Use the specified port from command-line arguments
	address := os.Args[1]
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("Failed to start HTTP server:", err)
		return
	}
	defer listener.Close()

	log.Println("HTTP server started on", address)
	// Start the HTTP server
	err = http.Serve(listener, nil)
	if err != nil {
		log.Fatal("HTTP server stopped with error:", err)
	}
}
