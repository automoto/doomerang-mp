package core

import (
	"log"
	"time"

	"github.com/leap-fish/necs/esync/srvsync"
)

// GameLoop runs the server game loop at a fixed tick rate
type GameLoop struct {
	server   *Server
	tickRate int
	running  bool
	stopChan chan struct{}
}

// NewGameLoop creates a new game loop
func NewGameLoop(server *Server, tickRate int) *GameLoop {
	return &GameLoop{
		server:   server,
		tickRate: tickRate,
		stopChan: make(chan struct{}),
	}
}

// Run starts the game loop
func (g *GameLoop) Run() {
	g.running = true
	tickDuration := time.Second / time.Duration(g.tickRate)
	ticker := time.NewTicker(tickDuration)
	defer ticker.Stop()

	log.Printf("Game loop started at %d ticks/second", g.tickRate)

	for {
		select {
		case <-g.stopChan:
			g.running = false
			log.Println("Game loop stopped")
			return
		case <-ticker.C:
			g.tick()
		}
	}
}

// Stop signals the game loop to stop
func (g *GameLoop) Stop() {
	close(g.stopChan)
}

// tick performs one game tick
func (g *GameLoop) tick() {
	// Process game systems here
	// TODO: Add physics, combat, enemy AI, etc.

	// Sync world state to all clients
	if err := srvsync.DoSync(); err != nil {
		log.Printf("Sync error: %v", err)
	}
}

// IsRunning returns whether the loop is currently running
func (g *GameLoop) IsRunning() bool {
	return g.running
}
