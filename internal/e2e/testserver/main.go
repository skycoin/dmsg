// Package main provides a simple HTTP server for e2e testing
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

var (
	port = flag.Int("port", 8086, "port to listen on")
)

func main() {
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "DMSG E2E Test Server\n")
		fmt.Fprintf(w, "Path: %s\n", r.URL.Path)
		fmt.Fprintf(w, "Method: %s\n", r.Method)
		fmt.Fprintf(w, "Host: %s\n", r.Host)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		for k, v := range query {
			fmt.Fprintf(w, "%s: %v\n", k, v)
		}
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting HTTP test server on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
		os.Exit(1)
	}
}
