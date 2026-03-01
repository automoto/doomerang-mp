package core

import (
	"log"
	"time"

	"github.com/leap-fish/necs/esync/srvsync"
)

type GameLoop struct {
	server    *Server
	botSystem *BotSystem
	tickRate  int
	running   bool
	stopChan  chan struct{}
}

func NewGameLoop(server *Server, tickRate int) *GameLoop {
	return &GameLoop{
		server:    server,
		botSystem: NewBotSystem(server),
		tickRate:  tickRate,
		stopChan:  make(chan struct{}),
	}
}

func (g *GameLoop) Run() {
	g.running = true
	ticker := time.NewTicker(time.Second / time.Duration(g.tickRate))
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

func (g *GameLoop) Stop() {
	close(g.stopChan)
}

func (g *GameLoop) tick() {
	dt := 1.0 / float64(g.tickRate)
	g.server.ProcessCommands()
	g.server.Match().Update(dt)
	g.botSystem.Update()
	g.server.updatePhysics()
	g.server.updateCombat()

	if err := srvsync.DoSync(); err != nil {
		log.Printf("Sync error: %v", err)
	}
}
