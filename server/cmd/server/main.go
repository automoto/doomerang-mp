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
	name := flag.String("name", "Doomerang Server", "Server display name")
	version := flag.String("version", "", "Required client version (empty = accept any)")
	moveSpeed := flag.Float64("movespeed", 3.0, "Player movement speed")
	master := flag.String("master", "", "Master server URL (e.g. http://localhost:8080)")
	region := flag.String("region", "", "Server region for display")
	maxPlayers := flag.Int("maxplayers", 4, "Maximum players")
	address := flag.String("address", "localhost:7373", "Public address to advertise")
	flag.Parse()

	if err := protocol.RegisterComponents(); err != nil {
		log.Fatalf("Failed to register components: %v", err)
	}

	server := core.NewServer(*tickRate, *name, *version, *moveSpeed)

	var reg *core.Registration
	if *master != "" {
		reg = core.NewRegistration(*master, *name, *address, *version, *region, *maxPlayers, server)
		reg.Start()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down server...")
		if reg != nil {
			reg.Stop()
		}
		server.Stop()
		os.Exit(0)
	}()

	log.Printf("Starting Doomerang server %q on port %d (tick rate: %d/s, version: %s)",
		*name, *port, *tickRate, *version)
	if err := server.Start(*port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
