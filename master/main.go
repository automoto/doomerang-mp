package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	port := flag.Int("port", 8080, "HTTP listen port")
	ttl := flag.Duration("ttl", 90*time.Second, "Server TTL before expiry")
	flag.Parse()

	reg := NewRegistry(*ttl)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /servers", ListServers(reg))
	mux.HandleFunc("POST /servers/register", RegisterServer(reg))
	mux.HandleFunc("POST /servers/heartbeat", Heartbeat(reg))
	mux.HandleFunc("GET /health", Health())

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[master] starting on %s (TTL=%s)", addr, *ttl)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[master] fatal: %v", err)
	}
}
