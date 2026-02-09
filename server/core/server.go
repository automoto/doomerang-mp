package core

import (
	"crypto/rand"
	"fmt"
	"log"
	"sync"

	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/leap-fish/necs/esync"
	"github.com/leap-fish/necs/esync/srvsync"
	"github.com/leap-fish/necs/router"
	"github.com/leap-fish/necs/transports"
	"github.com/yohamta/donburi"
)

// serverCmd is queued from router goroutines and executed on the game loop goroutine.
type serverCmd func()

type Server struct {
	world     donburi.World
	loop      *GameLoop
	transport *transports.WsServerTransport

	name    string
	version string

	levels      map[string]*ServerLevel
	levelNames  []string
	activeLevel *ServerLevel
	activeName  string

	playerPhysics map[donburi.Entity]*PlayerPhysics

	clientEntities map[*router.NetworkClient]donburi.Entity
	pendingClients map[*router.NetworkClient]bool
	mu             sync.RWMutex

	cmdCh chan serverCmd
}

func NewServer(tickRate int, name, version string, levels map[string]*ServerLevel, levelNames []string) *Server {
	if len(levelNames) == 0 {
		log.Fatal("NewServer: no levels provided")
	}
	world := donburi.NewWorld()

	s := &Server{
		world:          world,
		name:           name,
		version:        version,
		levels:         levels,
		levelNames:     levelNames,
		activeLevel:    levels[levelNames[0]],
		activeName:     levelNames[0],
		playerPhysics:  make(map[donburi.Entity]*PlayerPhysics),
		clientEntities: make(map[*router.NetworkClient]donburi.Entity),
		pendingClients: make(map[*router.NetworkClient]bool),
		cmdCh:          make(chan serverCmd, 64),
	}
	s.loop = NewGameLoop(s, tickRate)

	srvsync.UseEsync(world)
	s.setupRouterCallbacks()

	return s
}

// LevelNames returns the sorted list of available level names.
func (s *Server) LevelNames() []string {
	return s.levelNames
}

func (s *Server) Start(port uint) error {
	go s.loop.Run()

	s.transport = transports.NewWsServerTransport(port, "", nil)
	return s.transport.Start()
}

func (s *Server) Stop() {
	s.loop.Stop()
}

func (s *Server) ProcessCommands() {
	for {
		select {
		case cmd := <-s.cmdCh:
			cmd()
		default:
			return
		}
	}
}

func (s *Server) PlayerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clientEntities)
}

func (s *Server) setupRouterCallbacks() {
	router.OnConnect(func(client *router.NetworkClient) {
		s.onConnect(client)
	})

	router.OnDisconnect(func(client *router.NetworkClient, err error) {
		s.onDisconnect(client, err)
	})

	router.On(func(client *router.NetworkClient, req messages.JoinRequest) {
		s.onJoinRequest(client, req)
	})

	router.On(func(client *router.NetworkClient, input messages.PlayerInput) {
		s.onPlayerInput(client, input)
	})

	router.OnError(func(client *router.NetworkClient, err error) {
		log.Printf("Client error: %v", err)
	})
}

func (s *Server) onConnect(client *router.NetworkClient) {
	log.Printf("Client connected: %s (pending join)", client.Id())

	s.mu.Lock()
	s.pendingClients[client] = true
	s.mu.Unlock()
}

func (s *Server) onJoinRequest(client *router.NetworkClient, req messages.JoinRequest) {
	s.mu.Lock()
	isPending := s.pendingClients[client]
	s.mu.Unlock()

	if !isPending {
		log.Printf("Join request from non-pending client %s, ignoring", client.Id())
		return
	}

	if s.version != "" && req.Version != s.version {
		log.Printf("Client %s version mismatch: got %q, want %q", client.Id(), req.Version, s.version)
		_ = client.SendMessage(messages.JoinRejected{
			Reason: fmt.Sprintf("Version mismatch: server=%s client=%s", s.version, req.Version),
		})
		return
	}

	s.cmdCh <- func() {
		// Switch active level if requested and no players connected yet
		if req.Level != "" && len(s.clientEntities) == 0 {
			if lvl, ok := s.levels[req.Level]; ok {
				s.activeLevel = lvl
				s.activeName = req.Level
				log.Printf("Switched active level to %q", req.Level)
			}
		}
		s.spawnPlayer(client, req)
	}
}

// spawnPlayer must be called on the game loop goroutine.
func (s *Server) spawnPlayer(client *router.NetworkClient, req messages.JoinRequest) {
	// Pick spawn point round-robin by player count
	spawnX, spawnY := 100.0, 100.0
	if len(s.activeLevel.SpawnPoints) > 0 {
		idx := len(s.clientEntities) % len(s.activeLevel.SpawnPoints)
		sp := s.activeLevel.SpawnPoints[idx]
		spawnX, spawnY = sp.X, sp.Y
	}

	entity := s.world.Create(
		netcomponents.NetPosition,
		netcomponents.NetVelocity,
		netcomponents.NetPlayerState,
	)

	entry := s.world.Entry(entity)
	netcomponents.NetPosition.Set(entry, &netcomponents.NetPositionData{X: spawnX, Y: spawnY})
	netcomponents.NetVelocity.Set(entry, &netcomponents.NetVelocityData{})
	netcomponents.NetPlayerState.Set(entry, &netcomponents.NetPlayerStateData{
		Direction: 1,
		Health:    100,
	})

	// Create server-side physics for this player
	pp := newPlayerPhysics(s.activeLevel, spawnX, spawnY)
	s.playerPhysics[entity] = pp

	// NOTE: Do not use WithInterp — InterpData is unregistered with esync.Mapper,
	// causing buildEntityState to silently skip the entire entity.
	err := srvsync.NetworkSync(s.world, &entity,
		netcomponents.NetPosition,
		netcomponents.NetVelocity,
		netcomponents.NetPlayerState,
	)
	if err != nil {
		log.Printf("Failed to setup network sync for player: %v", err)
		_ = client.SendMessage(messages.JoinRejected{Reason: "internal server error"})
		return
	}

	networkID := esync.GetNetworkId(s.world.Entry(entity))

	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	reconnectToken := fmt.Sprintf("%x", tokenBytes)

	s.mu.Lock()
	delete(s.pendingClients, client)
	s.clientEntities[client] = entity
	s.mu.Unlock()

	_ = client.SendMessage(messages.JoinAccepted{
		NetworkID:      *networkID,
		ReconnectToken: reconnectToken,
		ServerName:     s.name,
		TickRate:       s.loop.tickRate,
		Level:          s.activeName,
		Levels:         s.levelNames,
	})

	log.Printf("Player %q joined as entity networkID=%d (client %s)",
		req.PlayerName, *networkID, client.Id())
}

func (s *Server) onDisconnect(client *router.NetworkClient, err error) {
	if err != nil {
		log.Printf("Client %s disconnected: %v", client.Id(), err)
	} else {
		log.Printf("Client %s disconnected", client.Id())
	}

	s.mu.Lock()
	delete(s.pendingClients, client)
	entity, exists := s.clientEntities[client]
	if exists {
		delete(s.clientEntities, client)
	}
	s.mu.Unlock()

	if !exists {
		return
	}

	s.cmdCh <- func() {
		if pp, ok := s.playerPhysics[entity]; ok {
			removePlayerPhysics(s.activeLevel, pp)
			delete(s.playerPhysics, entity)
		}
		if s.world.Valid(entity) {
			s.world.Remove(entity)
			log.Printf("Player entity removed for client %s", client.Id())
		}
	}
}

func (s *Server) onPlayerInput(client *router.NetworkClient, input messages.PlayerInput) {
	s.mu.RLock()
	entity, exists := s.clientEntities[client]
	s.mu.RUnlock()

	if !exists {
		return
	}

	s.cmdCh <- func() {
		pp, ok := s.playerPhysics[entity]
		if !ok {
			return
		}
		pp.Direction = input.Direction
		pp.JumpPressed = input.Actions[netconfig.ActionJump]

		// Update facing direction in NetPlayerState
		if input.Direction != 0 && s.world.Valid(entity) {
			entry := s.world.Entry(entity)
			state := netcomponents.NetPlayerState.Get(entry)
			state.Direction = input.Direction
		}
	}
}
