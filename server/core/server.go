package core

import (
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/leap-fish/necs/esync"
	"github.com/leap-fish/necs/esync/srvsync"
	"github.com/leap-fish/necs/router"
	"github.com/leap-fish/necs/transports"
	"github.com/yohamta/donburi"
)

const (
	defaultDrainTimeout = 30 * time.Second
	drainPollInterval   = 25 * time.Millisecond
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

	playerPhysics    map[donburi.Entity]*PlayerPhysics
	boomerangPhysics map[donburi.Entity]*BoomerangPhysics
	playerBoomerangs map[donburi.Entity]donburi.Entity // player → active boomerang

	clientEntities   map[*router.NetworkClient]donburi.Entity
	pendingClients   map[*router.NetworkClient]bool
	clientNetworkIDs map[*router.NetworkClient]uint32
	networkIDClients map[uint32]*router.NetworkClient
	// ggscaleTokens is keyed by netID and holds each player's ggscale
	// session JWT, captured from JoinRequest. Used at match end to
	// submit scores via Leaderboards.SubmitFor.
	ggscaleTokens map[uint32]string
	matchEndHook  MatchEndHook
	match         *ServerMatch
	mu            sync.RWMutex

	cmdCh chan serverCmd

	// matchInProgress is true between ServerMatch.startMatch and endMatch.
	// Drain polls it to wait out an in-flight match before stopping the loop.
	matchInProgress atomic.Bool

	// draining is set once Drain() begins; onJoinRequest checks it to
	// reject new players with "server draining".
	draining     atomic.Bool
	drainOnce    sync.Once
	drainDone    chan struct{}
	drainTimeout time.Duration // 0 means defaultDrainTimeout
}

// MatchEndHook is invoked once per match end with the final scores and
// the per-player ggscale session tokens captured at join time. The
// dedicated game-server binary supplies a hook that calls
// Leaderboards.SubmitFor; tests/dev binaries leave it nil.
type MatchEndHook func(scores map[uint32]int, ggscaleTokens map[uint32]string)

func NewServer(tickRate int, name, version string, levels map[string]*ServerLevel, levelNames []string) *Server {
	if len(levelNames) == 0 {
		log.Fatal("NewServer: no levels provided")
	}
	world := donburi.NewWorld()

	s := &Server{
		world:            world,
		name:             name,
		version:          version,
		levels:           levels,
		levelNames:       levelNames,
		activeLevel:      levels[levelNames[0]],
		activeName:       levelNames[0],
		playerPhysics:    make(map[donburi.Entity]*PlayerPhysics),
		boomerangPhysics: make(map[donburi.Entity]*BoomerangPhysics),
		playerBoomerangs: make(map[donburi.Entity]donburi.Entity),
		clientEntities:   make(map[*router.NetworkClient]donburi.Entity),
		pendingClients:   make(map[*router.NetworkClient]bool),
		clientNetworkIDs: make(map[*router.NetworkClient]uint32),
		networkIDClients: make(map[uint32]*router.NetworkClient),
		ggscaleTokens:    make(map[uint32]string),
		cmdCh:            make(chan serverCmd, 64),
		drainDone:        make(chan struct{}),
	}
	s.loop = NewGameLoop(s, tickRate)

	srvsync.UseEsync(world)
	s.match = NewServerMatch(s)
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

// Drain stops accepting new player joins, waits for any in-progress
// match to end (bounded by drainTimeout, default 30 s), then stops the
// game loop. Safe to call concurrently and multiple times — the first
// caller performs the drain; subsequent callers block until it finishes.
//
// Wired into both the Agones Shutdown watcher and the SIGTERM handler so
// the same shutdown path runs whether Agones triggers it or a local
// signal does.
func (s *Server) Drain() {
	s.draining.Store(true)
	s.drainOnce.Do(func() {
		defer close(s.drainDone)
		s.waitForMatchEnd()
		s.Stop()
	})
	<-s.drainDone
}

func (s *Server) waitForMatchEnd() {
	if !s.matchInProgress.Load() {
		return
	}
	timeout := s.drainTimeout
	if timeout == 0 {
		timeout = defaultDrainTimeout
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	poll := time.NewTicker(drainPollInterval)
	defer poll.Stop()
	for {
		select {
		case <-deadline.C:
			log.Printf("[drain] timeout (%v) elapsed while waiting for match end; stopping anyway", timeout)
			return
		case <-poll.C:
			if !s.matchInProgress.Load() {
				return
			}
		}
	}
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

func (s *Server) World() donburi.World {
	return s.world
}

func (s *Server) ActiveLevel() *ServerLevel {
	return s.activeLevel
}

func (s *Server) Match() *ServerMatch {
	return s.match
}

// SetMatchEndHook installs f as the callback that ServerMatch invokes
// at match end. Wire to a function that submits leaderboard scores via
// ggscale.Leaderboards.SubmitFor. Pass nil to clear.
func (s *Server) SetMatchEndHook(f MatchEndHook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchEndHook = f
}

// snapshotGgscaleTokens returns a copy of the netID→session-token map.
// Called from match-end paths so the hook can run without holding the
// server lock.
func (s *Server) snapshotGgscaleTokens() map[uint32]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[uint32]string, len(s.ggscaleTokens))
	for k, v := range s.ggscaleTokens {
		out[k] = v
	}
	return out
}

// invokeMatchEndHook is called by ServerMatch.endMatch with the final
// score map. Looks up the installed hook under the lock then runs it
// outside — the hook makes network calls.
func (s *Server) invokeMatchEndHook(scores map[uint32]int) {
	s.mu.RLock()
	hook := s.matchEndHook
	s.mu.RUnlock()
	if hook == nil {
		return
	}
	hook(scores, s.snapshotGgscaleTokens())
}

func (s *Server) GetPlayerPhysics(entity donburi.Entity) *PlayerPhysics {
	return s.playerPhysics[entity]
}

func (s *Server) SpawnBot(name string, difficulty cfg.BotDifficulty) {
	s.cmdCh <- func() {
		s.spawnBot(name, difficulty)
	}
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

	router.On(func(client *router.NetworkClient, action messages.LobbyAction) {
		s.onLobbyAction(client, action)
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

	if s.draining.Load() {
		log.Printf("Client %s rejected: server draining", client.Id())
		_ = client.SendMessage(messages.JoinRejected{Reason: "server draining"})
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
		Health:    cfg.Player.Health,
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
	s.clientNetworkIDs[client] = uint32(*networkID)
	s.networkIDClients[uint32(*networkID)] = client
	if req.GgscaleSessionToken != "" {
		s.ggscaleTokens[uint32(*networkID)] = req.GgscaleSessionToken
	}
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

	// Assign to lobby slot
	s.cmdCh <- func() {
		s.match.OnLobbyAction(uint32(*networkID), messages.LobbyAction{
			Action: "pick_slot",
			Value:  s.match.FirstEmptySlot(),
			String: req.PlayerName,
		})
	}
}

// broadcastEvent sends a message to all connected clients.
func (s *Server) broadcastEvent(msg any) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for client := range s.clientEntities {
		_ = client.SendMessage(msg)
	}
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
	if nid, ok := s.clientNetworkIDs[client]; ok {
		delete(s.networkIDClients, nid)
		delete(s.clientNetworkIDs, client)
	}
	s.mu.Unlock()

	if !exists {
		return
	}

	s.cmdCh <- func() {
		// Destroy active boomerang owned by this player
		if bEntity, ok := s.playerBoomerangs[entity]; ok {
			s.destroyBoomerang(bEntity)
		}

		if pp, ok := s.playerPhysics[entity]; ok {
			removePlayerPhysics(s.activeLevel, pp)
			delete(s.playerPhysics, entity)
		}

		// Update lobby state
		entry := s.world.Entry(entity)
		if nid := esync.GetNetworkId(entry); nid != nil {
			s.match.OnDisconnect(uint32(*nid))
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
		pp.AttackPressed = input.Actions[netconfig.ActionAttack]
		pp.BoomerangPressed = input.Actions[netconfig.ActionBoomerang]
		pp.MoveUpPressed = input.Actions[netconfig.ActionMoveUp]
		pp.CrouchPressed = input.Actions[netconfig.ActionCrouch]
		pp.LastInputSeq = input.Sequence

		// Update facing direction in NetPlayerState
		if input.Direction != 0 && s.world.Valid(entity) {
			entry := s.world.Entry(entity)
			state := netcomponents.NetPlayerState.Get(entry)
			state.Direction = input.Direction
		}
	}
}

func (s *Server) onLobbyAction(client *router.NetworkClient, action messages.LobbyAction) {
	s.mu.RLock()
	entity, exists := s.clientEntities[client]
	s.mu.RUnlock()

	if !exists {
		return
	}

	s.cmdCh <- func() {
		entry := s.world.Entry(entity)
		nid := esync.GetNetworkId(entry)
		if nid == nil {
			return
		}
		s.match.OnLobbyAction(uint32(*nid), action)
	}
}

func (s *Server) spawnBot(name string, difficulty cfg.BotDifficulty) {
	// Pick spawn point
	spawnX, spawnY := 100.0, 100.0
	if len(s.activeLevel.SpawnPoints) > 0 {
		idx := (len(s.clientEntities) + s.botCount()) % len(s.activeLevel.SpawnPoints)
		sp := s.activeLevel.SpawnPoints[idx]
		spawnX, spawnY = sp.X, sp.Y
	}

	entity := s.world.Create(
		netcomponents.NetPosition,
		netcomponents.NetVelocity,
		netcomponents.NetPlayerState,
		components.Player,
		components.PlayerInput,
		components.Bot,
	)

	entry := s.world.Entry(entity)
	netcomponents.NetPosition.Set(entry, &netcomponents.NetPositionData{X: spawnX, Y: spawnY})
	netcomponents.NetVelocity.Set(entry, &netcomponents.NetVelocityData{})
	netcomponents.NetPlayerState.Set(entry, &netcomponents.NetPlayerStateData{
		Direction: 1,
		Health:    cfg.Player.Health,
		IsBot:     true,
	})

	playerData := components.Player.Get(entry)
	playerData.PlayerIndex = 4 + s.botCount() // Bots indices start at 4

	botData := components.Bot.Get(entry)
	botData.Difficulty = difficulty
	// Initialize bot behavior from config
	if config, ok := cfg.Bot.Difficulties[difficulty]; ok {
		botData.ReactionDelay = config.ReactionDelay
		botData.AttackRange = config.AttackRange
		botData.RetreatThreshold = config.RetreatThreshold
	}

	// Create server-side physics for this bot
	pp := newPlayerPhysics(s.activeLevel, spawnX, spawnY)
	s.playerPhysics[entity] = pp

	err := srvsync.NetworkSync(s.world, &entity,
		netcomponents.NetPosition,
		netcomponents.NetVelocity,
		netcomponents.NetPlayerState,
	)
	if err != nil {
		log.Printf("Failed to setup network sync for bot: %v", err)
		return
	}

	networkID := esync.GetNetworkId(entry)
	log.Printf("Bot %q spawned as entity networkID=%d", name, *networkID)
}

func (s *Server) ClearAllPlayers() {
	// First destroy all boomerangs
	for bEntity := range s.boomerangPhysics {
		s.destroyBoomerang(bEntity)
	}

	// Remove all player entities and their physics
	for entity, pp := range s.playerPhysics {
		removePlayerPhysics(s.activeLevel, pp)
		if s.world.Valid(entity) {
			s.world.Remove(entity)
		}
	}

	// Clear maps
	clear(s.playerPhysics)
	clear(s.boomerangPhysics)
	clear(s.playerBoomerangs)
	// We do NOT clear clientEntities because we want to keep the connection -> entity mapping
	// but we need to update the entity in that map if we spawn new ones.
}

func (s *Server) SpawnPlayerAtSlot(slotIdx int, slot messages.LobbySlot) {
	spawnX, spawnY := 100.0, 100.0
	if len(s.activeLevel.SpawnPoints) > 0 {
		idx := slotIdx % len(s.activeLevel.SpawnPoints)
		sp := s.activeLevel.SpawnPoints[idx]
		spawnX, spawnY = sp.X, sp.Y
	}

	if slot.Type == 1 { // Human
		entity := s.world.Create(
			netcomponents.NetPosition,
			netcomponents.NetVelocity,
			netcomponents.NetPlayerState,
			components.Player,
			components.PlayerInput,
		)

		entry := s.world.Entry(entity)
		netcomponents.NetPosition.Set(entry, &netcomponents.NetPositionData{X: spawnX, Y: spawnY})
		netcomponents.NetVelocity.Set(entry, &netcomponents.NetVelocityData{})
		netcomponents.NetPlayerState.Set(entry, &netcomponents.NetPlayerStateData{
			Direction:   1,
			Health:      cfg.Player.Health,
			Lives:       cfg.Match.LivesPerRound,
			PlayerIndex: slotIdx,
		})

		playerData := components.Player.Get(entry)
		playerData.PlayerIndex = slotIdx

		pp := newPlayerPhysics(s.activeLevel, spawnX, spawnY)
		s.playerPhysics[entity] = pp

		_ = srvsync.NetworkSync(s.world, &entity,
			netcomponents.NetPosition,
			netcomponents.NetVelocity,
			netcomponents.NetPlayerState,
		)

		// Update mapping in clientEntities
		s.mu.Lock()
		if client, ok := s.networkIDClients[slot.PlayerID]; ok {
			s.clientEntities[client] = entity
			log.Printf("Re-associated client %s (nid=%d) with new entity for slot %d", client.Id(), slot.PlayerID, slotIdx)
		} else {
			log.Printf("Warning: Could not find client for nid=%d during slot spawning", slot.PlayerID)
		}
		s.mu.Unlock()

	} else if slot.Type == 2 { // Bot
		entity := s.world.Create(
			netcomponents.NetPosition,
			netcomponents.NetVelocity,
			netcomponents.NetPlayerState,
			components.Player,
			components.PlayerInput,
			components.Bot,
		)

		entry := s.world.Entry(entity)
		netcomponents.NetPosition.Set(entry, &netcomponents.NetPositionData{X: spawnX, Y: spawnY})
		netcomponents.NetVelocity.Set(entry, &netcomponents.NetVelocityData{})
		netcomponents.NetPlayerState.Set(entry, &netcomponents.NetPlayerStateData{
			Direction:   1,
			Health:      cfg.Player.Health,
			Lives:       cfg.Match.LivesPerRound,
			PlayerIndex: slotIdx,
			IsBot:       true,
		})

		playerData := components.Player.Get(entry)
		playerData.PlayerIndex = slotIdx

		botData := components.Bot.Get(entry)
		if config, ok := cfg.Bot.Difficulties[cfg.BotDifficulty(slot.Difficulty)]; ok {
			botData.ReactionDelay = config.ReactionDelay
			botData.AttackRange = config.AttackRange
			botData.RetreatThreshold = config.RetreatThreshold
		}

		pp := newPlayerPhysics(s.activeLevel, spawnX, spawnY)
		s.playerPhysics[entity] = pp

		_ = srvsync.NetworkSync(s.world, &entity,
			netcomponents.NetPosition,
			netcomponents.NetVelocity,
			netcomponents.NetPlayerState,
		)

		// Store bot's network ID into slot for round system lookups
		if nid := esync.GetNetworkId(s.world.Entry(entity)); nid != nil {
			s.match.Slots[slotIdx].PlayerID = uint32(*nid)
		}
	}
}

func (s *Server) botCount() int {
	count := 0
	components.Bot.Each(s.world, func(entry *donburi.Entry) {
		count++
	})
	return count
}
