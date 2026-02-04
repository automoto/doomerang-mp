package core

import (
	"log"
	"sync"

	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/leap-fish/necs/esync/srvsync"
	"github.com/leap-fish/necs/router"
	"github.com/leap-fish/necs/transports"
	"github.com/yohamta/donburi"
)

// Server manages the game state and client connections
type Server struct {
	world     donburi.World
	loop      *GameLoop
	transport *transports.WsServerTransport

	// Track which network client owns which entity
	clientEntities map[*router.NetworkClient]donburi.Entity
	mu             sync.RWMutex
}

// NewServer creates a new game server
func NewServer(tickRate int) *Server {
	world := donburi.NewWorld()

	s := &Server{
		world:          world,
		clientEntities: make(map[*router.NetworkClient]donburi.Entity),
	}
	s.loop = NewGameLoop(s, tickRate)

	// Set up the world for esync
	srvsync.UseEsync(world)

	// Register router callbacks
	s.setupRouterCallbacks()

	return s
}

// Start begins the server on the given port
func (s *Server) Start(port uint) error {
	// Start game loop
	go s.loop.Run()

	// Create and start WebSocket transport
	s.transport = transports.NewWsServerTransport(port, "", nil)
	return s.transport.Start()
}

// Stop gracefully shuts down the server
func (s *Server) Stop() {
	s.loop.Stop()
}

func (s *Server) setupRouterCallbacks() {
	// Handle new connections
	router.OnConnect(func(client *router.NetworkClient) {
		s.onConnect(client)
	})

	// Handle disconnections
	router.OnDisconnect(func(client *router.NetworkClient, err error) {
		s.onDisconnect(client, err)
	})

	// Handle player input messages
	router.On(func(client *router.NetworkClient, input messages.PlayerInput) {
		s.onPlayerInput(client, input)
	})

	// Handle errors
	router.OnError(func(client *router.NetworkClient, err error) {
		log.Printf("Client error: %v", err)
	})
}

func (s *Server) onConnect(client *router.NetworkClient) {
	log.Printf("Client connected: %s", client.Id())

	// Create player entity with network components
	entity := s.world.Create(
		netcomponents.NetPosition,
		netcomponents.NetVelocity,
		netcomponents.NetPlayerState,
	)

	entry := s.world.Entry(entity)

	// Set initial position (spawn point - TODO: use proper spawn points)
	netcomponents.NetPosition.Set(entry, &netcomponents.NetPositionData{
		X: 100,
		Y: 100,
	})

	// Set initial velocity
	netcomponents.NetVelocity.Set(entry, &netcomponents.NetVelocityData{
		SpeedX: 0,
		SpeedY: 0,
	})

	// Set initial player state
	netcomponents.NetPlayerState.Set(entry, &netcomponents.NetPlayerStateData{
		Direction: 1,
		Health:    100,
		IsLocal:   false,
	})

	// Mark entity for network sync with interpolation for position
	err := srvsync.NetworkSync(s.world, &entity,
		srvsync.WithInterp(netcomponents.NetPosition, netcomponents.NetVelocity),
		netcomponents.NetPlayerState,
	)
	if err != nil {
		log.Printf("Failed to setup network sync for player: %v", err)
		return
	}

	// Track the client-entity mapping
	s.mu.Lock()
	s.clientEntities[client] = entity
	s.mu.Unlock()

	log.Printf("Player spawned for client %s", client.Id())
}

func (s *Server) onDisconnect(client *router.NetworkClient, err error) {
	if err != nil {
		log.Printf("Client %s disconnected with error: %v", client.Id(), err)
	} else {
		log.Printf("Client %s disconnected", client.Id())
	}

	// Remove player entity
	s.mu.Lock()
	entity, exists := s.clientEntities[client]
	if exists {
		delete(s.clientEntities, client)
	}
	s.mu.Unlock()

	if exists && s.world.Valid(entity) {
		s.world.Remove(entity)
		log.Printf("Player entity removed for client %s", client.Id())
	}
}

func (s *Server) onPlayerInput(client *router.NetworkClient, input messages.PlayerInput) {
	s.mu.RLock()
	entity, exists := s.clientEntities[client]
	s.mu.RUnlock()

	if !exists || !s.world.Valid(entity) {
		return
	}

	entry := s.world.Entry(entity)

	// Store input for processing in game loop
	// For now, just update direction based on input
	pos := netcomponents.NetPosition.Get(entry)
	vel := netcomponents.NetVelocity.Get(entry)
	state := netcomponents.NetPlayerState.Get(entry)

	// Simple movement based on direction
	moveSpeed := 3.0
	if input.Direction != 0 {
		vel.SpeedX = float64(input.Direction) * moveSpeed
		state.Direction = input.Direction
	} else {
		vel.SpeedX = 0
	}

	// Apply velocity to position
	pos.X += vel.SpeedX
	pos.Y += vel.SpeedY
}

// World returns the ECS world
func (s *Server) World() donburi.World {
	return s.world
}

// PlayerCount returns the number of connected players
func (s *Server) PlayerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clientEntities)
}
