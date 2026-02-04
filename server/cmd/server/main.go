package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/automoto/doomerang-mp/server/core"
	"github.com/automoto/doomerang-mp/shared/protocol"
)

func main() {
	port := flag.Uint("port", 7373, "Server port")
	tickRate := flag.Int("tickrate", 20, "Server tick rate (updates per second)")
	flag.Parse()

	// Register network components
	if err := protocol.RegisterComponents(); err != nil {
		log.Fatalf("Failed to register components: %v", err)
	}

	// Create and configure server
	server := core.NewServer(*tickRate)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down server...")
		server.Stop()
		os.Exit(0)
	}()

	// Start server
	log.Printf("Starting Doomerang server on port %d (tick rate: %d/s)", *port, *tickRate)
	if err := server.Start(*port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
