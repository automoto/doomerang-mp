# Multiplayer Feature Plan: necs Integration

## Goal
Add competitive PvP multiplayer to Doomerang using the [necs](https://github.com/leap-fish/necs) networking library with client-side prediction and WASM browser support.

---

## Current Progress

**Completed:**
- Milestone 1 - Foundation & Shared Code ✓
- Milestone 2 - Basic Server ✓

**Next Step:** Milestone 2b - Single-Player Cleanup
- Remove level progression, checkpoints, local save/load
- Update pause menu and game over for multiplayer

**Then:** Milestone 3 - Server Browser & Connection

**Notes:**
- Updated necs to v0.0.5 (commit 82c5928) which includes full srvsync, clisync, router, and transports
- `esync.RegisterComponent()` now supports `esync.WithInterpFn()` for client-side interpolation
- Server uses 20 tick/sec by default (configurable via `--tickrate` flag)
- All import paths updated to `github.com/automoto/doomerang-mp` module name

**Files Created (Milestone 1):**
- `shared/netcomponents/position.go` - NetPosition + LerpNetPosition
- `shared/netcomponents/velocity.go` - NetVelocity + LerpNetVelocity
- `shared/netcomponents/playerstate.go` - NetPlayerState
- `shared/netcomponents/boomerang.go` - NetBoomerang + LerpNetBoomerang
- `shared/netcomponents/enemy.go` - NetEnemy + LerpNetEnemy
- `shared/netcomponents/gamestate.go` - NetGameState + MatchState enum
- `shared/protocol/register.go` - RegisterComponents() with sync IDs 10-15 + interpolation
- `shared/messages/input.go` - PlayerInput message type
- `shared/messages/events.go` - HitEvent, DeathEvent, SpawnEvent, etc.

**Files Created (Milestone 2):**
- `server/cmd/server/main.go` - Server entry point with --port and --tickrate flags
- `server/core/server.go` - Server struct, WebSocket setup, player spawn/despawn
- `server/core/loop.go` - Tick-based game loop with srvsync.DoSync()

---

## Milestone 1: Foundation & Shared Code

### Design
Create the shared code structure that both server and client will use. Define network components and message types with consistent serialization IDs.

### Directory Structure
```
server/                 # Dedicated game server (new)
  cmd/server/           # Entry point
  core/                 # Server loop, connection handling
  systems/              # Server-side game logic
shared/                 # Shared between client/server (new)
  netcomponents/        # Network-synced component definitions
  messages/             # Network message types (input, events)
  protocol/             # Sync ID registration
network/                # Client network code (new)
  client.go             # Connection management
  prediction.go         # Client-side prediction
```

### Component Sync IDs
| ID | Component | Interpolated | Purpose |
|----|-----------|--------------|---------|
| 1 | NetworkId | No | Reserved by necs |
| 10 | NetPosition | Yes | X, Y coordinates |
| 11 | NetVelocity | No | SpeedX, SpeedY for prediction |
| 12 | NetPlayerState | No | StateID, Direction, Health |
| 13 | NetBoomerang | Yes | Position, state, owner ID |
| 14 | NetEnemy | Yes | Position, state, health |
| 15 | NetGameState | No | Scores, timer, match state |

### Implementation Tasks
- [x] Add necs dependency to go.mod
- [x] Create `shared/` directory structure
- [x] Define `NetPosition` component (X, Y float64) + LerpNetPosition
- [x] Define `NetVelocity` component (SpeedX, SpeedY float64) + LerpNetVelocity
- [x] Define `NetPlayerState` component (StateID, Direction, Health, IsLocal)
- [x] Define `NetBoomerang` component (OwnerID, State, DistanceTraveled) + LerpNetBoomerang
- [x] Define `NetEnemy` component (TypeName, State, Health) + LerpNetEnemy
- [x] Define `NetGameState` singleton component (Scores map, Timer, MatchState)
- [x] Create `shared/protocol/register.go` with ID constants and registration function
- [x] Create `shared/messages/input.go` with PlayerInput message type
- [x] Create `shared/messages/events.go` with game event message types
- [x] Update all import paths from `github.com/automoto/doomerang` to `github.com/automoto/doomerang-mp`

**Status: COMPLETE** ✓

---

## Milestone 2: Basic Server

### Design
A headless game server that accepts WebSocket connections, manages the ECS world, and broadcasts entity state at a fixed tick rate.

### Behavior
1. Server starts and listens on configurable port (default: 7373)
2. When client connects: spawn player entity, assign NetworkId
3. Run game loop at 60 ticks/second
4. Each tick: process inputs → update physics → resolve combat → sync state
5. On disconnect: remove player entity, notify other clients

### Implementation Tasks
- [x] Create `server/cmd/server/main.go` entry point
- [x] Create `server/core/server.go` with Server struct
- [x] Implement WebSocket server setup with necs transports
- [x] Create `server/core/loop.go` with tick-based game loop
- [x] Register components with `srvsync.UseEsync(world)`
- [x] Implement `router.OnConnect` - spawn player entity
- [x] Implement `router.OnDisconnect` - remove player entity
- [x] Call `srvsync.DoSync()` each tick to broadcast state
- [x] Add server to Makefile: `make server` and `make run-server`

### Files Created
| File | Purpose |
|------|---------|
| `server/cmd/server/main.go` | Server entry point with flags |
| `server/core/server.go` | Server struct, connection handling, player spawn/despawn |
| `server/core/loop.go` | Game loop at configurable tick rate |

**Status: COMPLETE** ✓

---

## Milestone 2b: Single-Player Cleanup

### Design
Remove single-player only features and code that won't be used in the multiplayer version. This simplifies the codebase and clarifies what's client vs server responsibility.

### Features to Remove
| Feature | Files Affected | Reason |
|---------|---------------|--------|
| Level progression | `systems/levelcomplete.go`, `systems/finishline.go` | MP uses match-based rounds, not level progression |
| Checkpoints | `systems/checkpoint.go`, `systems/factory/checkpoint.go` | No checkpoints in PvP - respawn at spawn points |
| Pause menu (single-player) | `systems/pause.go` | MP can't pause - replace with disconnect/settings |
| Local save/load | `systems/persistence.go` | Server manages match state |
| Game over (single-player) | `scenes/gameover.go`, `systems/gameover.go` | Replace with match results screen |

### Files to Remove
- [ ] `systems/levelcomplete.go` - Level completion logic
- [ ] `systems/finishline.go` - Finish line detection
- [ ] `systems/factory/finishline.go` - Finish line factory
- [ ] `systems/checkpoint.go` - Checkpoint system
- [ ] `systems/factory/checkpoint.go` - Checkpoint factory
- [ ] `systems/persistence.go` - Local game save/load

### Files to Modify
- [ ] `systems/pause.go` - Convert to multiplayer menu (disconnect, settings)
- [ ] `scenes/gameover.go` - Convert to match results screen
- [ ] `systems/gameover.go` - Convert to match end logic
- [ ] `scenes/menu.go` - Remove "Continue" option, add "Multiplayer" button
- [ ] `archetypes/archetypes.go` - Remove Checkpoint, FinishLine archetypes
- [ ] `components/` - Remove checkpoint-related components if any

### Implementation Tasks
- [ ] Remove level progression systems and factories
- [ ] Remove checkpoint systems and factories
- [ ] Update pause menu for multiplayer (disconnect option)
- [ ] Convert game over to match results
- [ ] Update main menu (remove Continue, add Multiplayer)
- [ ] Clean up unused archetypes and components
- [ ] Update any remaining references
- [ ] Verify game still builds: `go build ./...`

---

## Milestone 3: Server Browser & Connection

### Design
Players can browse available servers from a master server, manually enter IP addresses, save favorites, and view recent servers. Supports password-protected private servers.

### Server Browser UI Flow
```
Main Menu
    └── Multiplayer
            ├── Server Browser (default tab)
            │       ├── [Refresh] button
            │       ├── Server list (sortable by name, players, ping, region)
            │       │       └── Each row: Name | Players (2/4) | Ping | Region | [Join]
            │       └── Filter: Region dropdown, Hide Full, Hide Empty
            │
            ├── Favorites (tab)
            │       └── Saved servers with quick join
            │
            ├── Recent (tab)
            │       └── Last 10 servers played
            │
            └── Direct Connect (tab)
                    ├── IP Address: [____________]
                    ├── Port: [7373___]
                    ├── Password: [____________] (optional)
                    └── [Connect] button
```

### Server Info Display
| Field | Source | Description |
|-------|--------|-------------|
| Name | Server config | e.g., "US East #1", "Bob's Private Server" |
| Players | Live query | "2/4" current/max |
| Ping | Client measured | RTT in milliseconds |
| Region | Server config | "US-East", "EU-West", "Asia", etc. |
| Password | Server flag | Lock icon if password required |

### Master Server Architecture
```
┌─────────────────┐         ┌─────────────────┐
│   Game Server   │────────▶│  Master Server  │◀────────┐
│   (registers)   │         │  (HTTP REST)    │         │
└─────────────────┘         └────────┬────────┘         │
                                     │                  │
                            ┌────────▼────────┐         │
                            │     Client      │─────────┘
                            │  (queries list) │
                            └─────────────────┘
```

**Master Server Endpoints:**
```
GET  /servers              - List all active servers
POST /servers/register     - Game server registers itself
POST /servers/heartbeat    - Game server sends heartbeat (every 30s)
DELETE /servers/{id}       - Game server deregisters on shutdown
```

**Server Registration Payload:**
```go
type ServerInfo struct {
    ID          string `json:"id"`          // Unique server ID
    Name        string `json:"name"`        // Display name
    Address     string `json:"address"`     // Public IP:port
    Region      string `json:"region"`      // Geographic region
    Players     int    `json:"players"`     // Current player count
    MaxPlayers  int    `json:"max_players"` // Max capacity
    HasPassword bool   `json:"has_password"`// Password protected?
    GameMode    string `json:"game_mode"`   // "pvp", "coop", etc.
    Version     string `json:"version"`     // Game version for compatibility
}
```

### Password Protection
```go
// Client sends password with join request
type JoinRequest struct {
    Password string `json:"password,omitempty"`
}

// Server validates before allowing connection
func (s *Server) OnConnect(client *NetworkClient) {
    if s.config.Password != "" {
        // Wait for JoinRequest message
        // Disconnect if wrong password
    }
}
```

### Local Storage (Favorites & History)
```go
// Stored in ~/.doomerang/servers.json (or browser localStorage for WASM)
type ServerStorage struct {
    Favorites []SavedServer `json:"favorites"`
    Recent    []SavedServer `json:"recent"`    // Max 10, FIFO
}

type SavedServer struct {
    Address   string    `json:"address"`
    Name      string    `json:"name"`
    Password  string    `json:"password,omitempty"` // Encrypted
    LastPlayed time.Time `json:"last_played"`
}
```

### Implementation Tasks

**Master Server (new service):**
- [ ] Create `master/` directory for master server service
- [ ] Implement HTTP REST API (Go + net/http or chi router)
- [ ] In-memory server registry with TTL (remove stale servers)
- [ ] Health endpoint for monitoring
- [ ] Deploy master server (separate from game servers)

**Game Server Registration:**
- [ ] Add `--name`, `--region`, `--password` flags to game server
- [ ] Register with master server on startup
- [ ] Send heartbeat every 30 seconds
- [ ] Deregister on graceful shutdown
- [ ] Handle master server unavailable (log warning, continue)

**Client Server Browser:**
- [ ] Create `scenes/serverbrowser.go` - main multiplayer menu
- [ ] Implement server list UI with columns and sorting
- [ ] Query master server for server list
- [ ] Ping each server for latency measurement
- [ ] Implement region filter and hide full/empty filters
- [ ] Show lock icon for password-protected servers

**Client Favorites & Recent:**
- [ ] Create `storage/servers.go` - local persistence
- [ ] Implement add/remove favorites
- [ ] Auto-save to recent on successful join
- [ ] Handle WASM (use localStorage instead of file)
- [ ] Encrypt stored passwords

**Client Direct Connect:**
- [ ] IP address input field with validation
- [ ] Port input with default (7373)
- [ ] Password input (hidden characters)
- [ ] Connect button with loading state
- [ ] Error handling (connection refused, wrong password, timeout)

**Password Protection:**
- [ ] Add password config to game server
- [ ] Implement password validation on join
- [ ] Send appropriate error message on wrong password
- [ ] Client prompts for password when joining protected server

### Files to Create
| File | Purpose |
|------|---------|
| `master/main.go` | Master server entry point |
| `master/registry.go` | Server registry with TTL |
| `master/handlers.go` | HTTP REST handlers |
| `scenes/serverbrowser.go` | Server browser UI scene |
| `storage/servers.go` | Local favorites/recent storage |
| `network/client.go` | WebSocket client wrapper |

### Files to Modify
| File | Changes |
|------|---------|
| `server/cmd/server/main.go` | Add name, region, password flags |
| `server/core/server.go` | Master server registration, password validation |

---

## Milestone 3b: Client Connection & Entity Sync

### Design
After selecting a server, client connects and receives entity state. Remote players appear and move based on server snapshots.

### Behavior
1. Client connects to selected server WebSocket
2. Client sends JoinRequest (with password if needed)
3. Server validates and spawns player entity
4. Server sends world snapshot with all synced entities
5. Client creates/updates entities from snapshot
6. Remote players render at server-provided positions

### Implementation Tasks
- [ ] Modify `scenes/world.go` to support networked mode
- [ ] Register components with `clisync.RegisterClient(world)`
- [ ] Add interpolation system for remote entities (`clisync.NewInterpolateSystem()`)
- [ ] Create lerp functions for NetPosition
- [ ] Distinguish local vs remote players in rendering
- [ ] Handle connection errors gracefully (retry, error message)
- [ ] Handle disconnection (return to server browser)
- [ ] Show "Connecting..." overlay during connection

### Files to Modify
| File | Changes |
|------|---------|
| `scenes/world.go` | Add network init, mode selection |
| `systems/factory/player.go` | Support creating networked players |
| `archetypes/archetypes.go` | Add network components to Player |

---

## Milestone 4: Input & Server-Side Movement

### Design
Client captures input and sends it to the server. Server processes input and moves players authoritatively.

### Behavior
1. Client captures input each frame
2. Client sends `PlayerInput` message with action states
3. Server receives input, applies to player entity
4. Server runs physics/movement for all players
5. Updated positions broadcast via DoSync

### PlayerInput Message
```go
type PlayerInput struct {
    Sequence   uint32              // For acknowledgment
    Actions    map[ActionID]bool   // Which actions are pressed
    Direction  int                 // -1 left, 0 none, 1 right
    Timestamp  int64               // Client timestamp
}
```

### Implementation Tasks
- [ ] Create `shared/messages/input.go` with PlayerInput struct
- [ ] Modify `systems/input.go` to send input when connected
- [ ] Create `server/systems/input.go` - receive and store inputs
- [ ] Create `server/systems/player.go` - apply input to movement
- [ ] Port physics constants to server
- [ ] Implement movement logic on server (gravity, collision)
- [ ] Store input sequence for reconciliation

---

## Milestone 5: Client-Side Prediction

### Design
For responsive controls, the local player moves immediately based on local input (prediction). When server state arrives, reconcile any differences.

### Behavior
1. Local input → immediately apply locally (predict)
2. Store input + predicted state with sequence number
3. Send input to server
4. Receive server snapshot with last acknowledged sequence
5. If server position differs from predicted:
   - Snap to server position if large difference
   - Interpolate if small difference
6. Replay unacknowledged inputs on top of server state

### Implementation Tasks
- [ ] Create `network/prediction.go` - prediction buffer
- [ ] Store input history (ring buffer, ~64 frames)
- [ ] Store predicted position for each input
- [ ] Apply local input immediately to local player
- [ ] On server snapshot: compare acknowledged position
- [ ] Implement reconciliation (snap vs interpolate threshold)
- [ ] Replay unacknowledged inputs after reconciliation
- [ ] Add config for prediction buffer size and snap threshold

---

## Milestone 6: Boomerang Sync

### Design
Boomerang throws are initiated by clients, validated by server, and synchronized to all players.

### Behavior
1. Client presses throw → send ThrowBoomerang message
2. Server validates (has boomerang, not already active)
3. Server spawns boomerang entity with NetworkId
4. Boomerang state synced to all clients
5. Collision/return handled authoritatively by server
6. On catch: server updates player state, removes boomerang entity

### Implementation Tasks
- [ ] Create `shared/messages/boomerang.go` with throw message
- [ ] Create `server/systems/boomerang.go` - server-side logic
- [ ] Modify client boomerang throw to send message when connected
- [ ] Handle boomerang entity creation/destruction on client
- [ ] Sync boomerang position with interpolation
- [ ] Handle boomerang collisions on server
- [ ] Sync hit events to clients

---

## Milestone 7: Combat Sync

### Design
All combat (punches, boomerang hits) is resolved by the server. Hit events are broadcast to clients.

### Behavior
1. Punch/attack: client sends AttackInput
2. Server creates hitbox, checks collisions
3. Hit registered: server applies damage, knockback
4. Server sends HitEvent to affected clients
5. Clients play hit effects/sounds

### Message Types
```go
type HitEvent struct {
    AttackerID  uint32
    TargetID    uint32
    Damage      int
    KnockbackX  float64
    KnockbackY  float64
}

type DeathEvent struct {
    VictimID    uint32
    KillerID    uint32
}
```

### Implementation Tasks
- [ ] Create `shared/messages/combat.go` with combat messages
- [ ] Create `server/systems/combat.go` - server-side combat
- [ ] Handle punch on server (hitbox creation, collision)
- [ ] Handle boomerang hits on server
- [ ] Broadcast HitEvent to clients
- [ ] Client receives HitEvent, plays effects
- [ ] Handle player death on server
- [ ] Broadcast DeathEvent, handle respawn

---

## Milestone 8: Enemy Sync

### Design
For PvP, enemies may be optional. If included, server manages all enemy AI and syncs state.

### Behavior
- Server runs enemy AI (patrol, chase, attack)
- Enemy positions/states synced to all clients
- Clients only render enemies (no local AI)
- Player-enemy combat resolved on server

### Implementation Tasks
- [ ] Port enemy AI to server
- [ ] Sync enemy positions with interpolation
- [ ] Handle enemy-player collisions on server
- [ ] Sync enemy deaths to clients
- [ ] Option to disable enemies in PvP mode

---

## Milestone 9: Game State & Match Flow

### Design
Server manages match state (waiting, playing, finished). Tracks scores and declares winner.

### Match States
| State | Behavior |
|-------|----------|
| Waiting | Waiting for minimum players |
| Countdown | Match starting in 3...2...1 |
| Playing | Active gameplay |
| Finished | Match over, show results |

### Implementation Tasks
- [ ] Create `NetGameState` singleton with match state
- [ ] Implement waiting room (player count threshold)
- [ ] Implement countdown timer
- [ ] Track kills/deaths per player
- [ ] Detect win condition (score limit or last standing)
- [ ] Broadcast match state changes
- [ ] Client UI for scores, timer, results

---

## Milestone 10: WASM Build

### Design
Build the client as WebAssembly for browser play. Server remains native Go.

### Behavior
- Same client code, different build target
- HTML page hosts WASM binary
- WebSocket works natively in browser
- Input via keyboard (no gamepad in browser)

### Implementation Tasks
- [ ] Create `build/wasm/index.html` wrapper page
- [ ] Copy `wasm_exec.js` from Go installation
- [ ] Add Makefile target: `make wasm`
- [ ] Handle WASM-specific input (keyboard only)
- [ ] Test in Chrome, Firefox, Safari
- [ ] Add loading indicator while WASM loads
- [ ] Configure CORS on server for browser clients

### Build Commands
```bash
# Native client
go build -o doomerang ./cmd/client

# WASM client
GOOS=js GOARCH=wasm go build -o build/wasm/doomerang.wasm .

# Server (always native)
go build -o doomerang-server ./server/cmd/server
```

---

## Config Values

```go
// config/config.go - add Network section
var Network = struct {
    ServerPort       int
    ServerAddress    string
    TickRate         int
    ViewDistance     float64
    PredictionBuffer int
    SnapThreshold    float64
    InterpDelay      float64
}{
    ServerPort:       7373,
    ServerAddress:    "localhost",
    TickRate:         60,
    ViewDistance:     800.0,
    PredictionBuffer: 64,
    SnapThreshold:    50.0,  // pixels
    InterpDelay:      100.0, // milliseconds
}

var Match = struct {
    MinPlayers    int
    MaxPlayers    int
    CountdownTime float64
    ScoreToWin    int
    RespawnDelay  float64
}{
    MinPlayers:    2,
    MaxPlayers:    4,
    CountdownTime: 3.0,
    ScoreToWin:    5,
    RespawnDelay:  2.0,
}

var MasterServer = struct {
    URL              string
    HeartbeatInterval time.Duration
    ServerTTL         time.Duration
    PingTimeout       time.Duration
}{
    URL:               "https://master.doomerang.io",  // Or your domain
    HeartbeatInterval: 30 * time.Second,
    ServerTTL:         90 * time.Second,  // Remove if no heartbeat for 90s
    PingTimeout:       2 * time.Second,
}

var ServerBrowser = struct {
    MaxRecentServers  int
    MaxFavorites      int
    RefreshInterval   time.Duration
    DefaultPort       int
}{
    MaxRecentServers: 10,
    MaxFavorites:     50,
    RefreshInterval:  30 * time.Second,
    DefaultPort:      7373,
}
```

---

## Files Summary

### New Files to Create
| File | Milestone | Purpose |
|------|-----------|---------|
| `shared/netcomponents/*.go` | 1 | Network component definitions |
| `shared/messages/*.go` | 1 | Network message types |
| `shared/protocol/register.go` | 1 | Component ID registration |
| `server/cmd/server/main.go` | 2 | Server entry point |
| `server/core/server.go` | 2 | Server struct |
| `server/core/loop.go` | 2 | Game loop |
| `server/systems/*.go` | 4-8 | Server-side systems |
| `master/main.go` | 3 | Master server entry point |
| `master/registry.go` | 3 | Server registry with TTL |
| `master/handlers.go` | 3 | HTTP REST handlers |
| `scenes/serverbrowser.go` | 3 | Server browser UI scene |
| `storage/servers.go` | 3 | Local favorites/recent storage |
| `network/client.go` | 3b | Client connection |
| `network/prediction.go` | 5 | Client prediction |
| `build/wasm/index.html` | 10 | WASM wrapper |

### Existing Files to Modify
| File | Milestone | Changes |
|------|-----------|---------|
| `go.mod` | 1 | Add necs dependency |
| `config/config.go` | 1 | Add Network/Match/MasterServer/ServerBrowser config |
| `scenes/world.go` | 3b | Add network mode |
| `scenes/menu.go` | 3 | Add "Multiplayer" button leading to server browser |
| `systems/input.go` | 4 | Send input to server |
| `systems/player.go` | 5 | Add prediction logic |
| `archetypes/archetypes.go` | 1 | Add network components |
| `Makefile` | 2, 10 | Add server/wasm/master targets |
| `server/cmd/server/main.go` | 3 | Add name, region, password flags |
| `server/core/server.go` | 3 | Master server registration, password validation |

---

## Verification

### Per-Milestone Testing
1. **M1**: Components serialize/deserialize correctly (unit test)
2. **M2**: Server starts, accepts connection, logs it
3. **M3**: Server browser shows servers from master server, can browse/filter/favorite
3. **M3 (master)**: Master server starts, game servers register, clients query list
3. **M3 (direct)**: Manual IP:port entry connects successfully
3. **M3 (password)**: Password-protected server rejects wrong password, accepts correct
3. **M3 (storage)**: Favorites persist after restart, recent servers tracked
4. **M3b**: Two clients connect via browser, see each other's placeholder
5. **M4**: Movement works via server (high latency visible)
6. **M5**: Movement feels responsive with prediction
7. **M6**: Boomerang throws sync between clients
8. **M7**: Hits register, damage syncs
9. **M8**: Enemies visible and move on all clients
10. **M9**: Full match flow works
11. **M10**: Browser client connects and plays (WASM uses localStorage for favorites)

### Integration Test
```bash
# Terminal 1: Start server
./doomerang-server

# Terminal 2: Start client 1
./doomerang --connect localhost:7373

# Terminal 3: Start client 2
./doomerang --connect localhost:7373

# Verify both players visible, can fight
```

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Physics non-determinism | Position desync | Server authoritative, client accepts corrections |
| Input delay feel | Poor UX | Client-side prediction |
| Resolv not thread-safe | Crashes | Single-threaded server game loop |
| WASM performance | Lag in browser | Profile, optimize, reduce sync rate if needed |
| Network latency | Poor hit registration | Server-side hit detection with lag compensation |

---

## Production Architecture (Scale: 10,000+ Concurrent Players)

### Architecture Overview

For a small DigitalOcean VPS to handle thousands of players, we need a **match-based architecture** rather than a single world server.

```
                                    ┌─────────────────────┐
                                    │   Load Balancer     │
                                    │   (DO App Platform  │
                                    │    or nginx)        │
                                    └──────────┬──────────┘
                                               │
                    ┌──────────────────────────┼──────────────────────────┐
                    │                          │                          │
           ┌────────▼────────┐       ┌────────▼────────┐       ┌────────▼────────┐
           │  Game Server 1  │       │  Game Server 2  │       │  Game Server N  │
           │  (Matches 1-50) │       │  (Matches 51-100)│       │  (Matches ...)  │
           └─────────────────┘       └─────────────────┘       └─────────────────┘
```

### Match-Based Design

Each match is an isolated game instance (2-4 players). Benefits:
- **Horizontal scaling**: Add more server instances as needed
- **Fault isolation**: One bad match doesn't affect others
- **Memory bounded**: Each match has predictable resource usage
- **No global state**: Easy to distribute across servers

```go
// server/core/match.go
type Match struct {
    ID          string
    World       donburi.World
    Players     map[string]*Player  // NetworkClient ID -> Player
    State       MatchState
    TickRate    int
    CreatedAt   time.Time

    mu          sync.RWMutex
    stopChan    chan struct{}
}

type MatchManager struct {
    matches     map[string]*Match
    maxMatches  int
    mu          sync.RWMutex
}
```

### Resource Budgeting (per 2GB RAM / 2 vCPU VPS)

**Detailed CPU Analysis (30 tick/sec per match):**
```
Physics simulation (resolv):  ~100µs/tick
Collision detection:          ~50µs/tick
Combat/state logic:           ~20µs/tick
necs serialization:           ~200µs/tick
────────────────────────────────────────────
Total per match:              ~370µs/tick × 30 = 11.1ms/sec
```

**Capacity per CPU core**: `1000ms / 11.1ms = ~90 matches = 360 players`

| Resource | Per Match (4p) | Per VPS (2 core) | Max Players |
|----------|----------------|------------------|-------------|
| CPU | 11ms/sec | 180 matches | 720 |
| Memory | ~5 MB | 400 matches | 1600 |
| Bandwidth | 18 KB/sec | unlimited | - |

**Bottleneck**: CPU (not memory or network)

**Realistic estimate (2-core $12/mo VPS)**:
- Conservative: 500-700 concurrent players
- Optimized: 800-1000 concurrent players

**Scaling Estimates:**

| Concurrent Players | VPS Needed | Monthly Cost |
|-------------------|------------|--------------|
| 500 | 1 | $12 |
| 2,000 | 3 | $36 |
| 5,000 | 6-7 | $72-84 |
| 10,000 | 12-15 | $144-180 |
| 50,000 | 60-70 | $720-840 |

For comparison: Agar.io and Slither.io handle 100K+ concurrent on similar architecture.

### Rate Limiting

```go
// server/middleware/ratelimit.go
type RateLimiter struct {
    // Per-IP connection rate (prevent connection floods)
    connLimiter *rate.Limiter  // 5 connections/sec per IP

    // Per-client message rate (prevent message spam)
    msgLimiters sync.Map       // client ID -> *rate.Limiter

    // Global server rate (circuit breaker)
    globalLimiter *rate.Limiter
}

// Config
var RateLimit = struct {
    ConnectionsPerIPPerSec  float64
    MessagesPerClientPerSec float64
    MaxBurstMessages        int
    GlobalMessagesPerSec    float64
    BanDurationMinutes      int
}{
    ConnectionsPerIPPerSec:  5,
    MessagesPerClientPerSec: 60,   // 60 inputs/sec max
    MaxBurstMessages:        10,
    GlobalMessagesPerSec:    50000,
    BanDurationMinutes:      5,
}
```

### Rate Limiting Implementation Tasks
- [ ] Create `server/middleware/ratelimit.go`
- [ ] Implement per-IP connection rate limiting
- [ ] Implement per-client message rate limiting
- [ ] Add temporary IP bans for repeat offenders
- [ ] Add global circuit breaker for server protection
- [ ] Log rate limit violations for monitoring

### Connection Management

```go
// server/core/connection.go
var Connection = struct {
    MaxConcurrentConnections int
    MaxConnectionsPerIP      int
    HandshakeTimeout         time.Duration
    IdleTimeout              time.Duration
    MaxMessageSize           int
    WriteBufferSize          int
    ReadBufferSize           int
}{
    MaxConcurrentConnections: 1000,  // per server instance
    MaxConnectionsPerIP:      4,     // prevent single IP hogging
    HandshakeTimeout:         10 * time.Second,
    IdleTimeout:              60 * time.Second,
    MaxMessageSize:           4096,  // 4KB max message
    WriteBufferSize:          1024,
    ReadBufferSize:           1024,
}
```

### Tick Rate Optimization

60 ticks/sec is expensive. For a VPS:

| Tick Rate | CPU/Match | Bandwidth/Player | Feel |
|-----------|-----------|------------------|------|
| 60 | High | ~15 KB/s | Excellent |
| 30 | Medium | ~8 KB/s | Good (recommended) |
| 20 | Low | ~5 KB/s | Acceptable |

**Recommendation**: 30 tick server, 60 FPS client with interpolation

```go
var Server = struct {
    TickRate         int
    MaxMatchDuration time.Duration
    MatchCleanupAge  time.Duration
}{
    TickRate:         30,  // 30 ticks/sec server
    MaxMatchDuration: 10 * time.Minute,
    MatchCleanupAge:  5 * time.Minute,
}
```

### Efficiency Optimizations (Reduce VPS Count by 30-50%)

These optimizations can push capacity from 700 to 1000+ players per VPS:

**1. Reduce Tick Rate (20 vs 30)**
```go
// 20 ticks/sec is sufficient for a 2D platformer
// Saves 33% CPU, client interpolates the difference
TickRate: 20  // Instead of 30
```
Impact: **33% more capacity**

**2. Hybrid Tick Rates**
```go
// Not everything needs high-frequency updates
PositionTickRate: 20  // Movement (high frequency)
StateTickRate:    5   // Health, score (low frequency)
EventTickRate:    0   // Combat hits (event-driven, not polled)
```
Impact: **20% bandwidth reduction**

**3. Object Pooling (Zero-Allocation Ticks)**
```go
// Reuse serialization buffers
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 4096)
    },
}

func (m *Match) Tick() {
    buf := bufferPool.Get().([]byte)
    defer bufferPool.Put(buf[:0])
    // ... use buf for serialization
}
```
Impact: **50% GC reduction, smoother performance**

**4. Delta Compression**
```go
// Only send components that changed since last tick
type EntityDelta struct {
    NetworkID    uint32
    ChangedMask  uint16           // Bitmask: which components changed
    Components   []ComponentDelta // Only the changed ones
}

// For a player standing still: send nothing
// For a player moving: send only position (not health, state, etc.)
```
Impact: **40-70% bandwidth reduction**

**5. Position Quantization**
```go
// Use fixed-point integers instead of float64
type NetPosition struct {
    X int16  // actual = X * 0.1, range: ±3276.8
    Y int16  // Saves 12 bytes per entity per tick
}

func QuantizePosition(x, y float64) NetPosition {
    return NetPosition{
        X: int16(x * 10),
        Y: int16(y * 10),
    }
}
```
Impact: **75% position data reduction**

**6. Spatial Interest Management**
```go
// Only sync entities within player's view + margin
ViewDistance:   400.0  // pixels
MarginDistance: 100.0  // buffer to prevent pop-in

// For a level with 100 entities, player sees ~20
// 80% entity sync reduction
```
Impact: **50-80% fewer entities synced**

**Combined Impact:**

| Optimization | CPU Savings | Bandwidth Savings |
|--------------|-------------|-------------------|
| 20 tick rate | 33% | 33% |
| Object pooling | 15% (smoother) | 0% |
| Delta compression | 10% | 50% |
| Quantization | 5% | 20% |
| Interest management | 20% | 60% |
| **Combined** | **~50%** | **~80%** |

**Optimized Capacity (2-core $12/mo VPS):**
- Before optimization: 500-700 players
- After optimization: **1000-1500 players**

**Optimized Scaling:**

| Concurrent Players | VPS (Unoptimized) | VPS (Optimized) | Savings |
|-------------------|-------------------|-----------------|---------|
| 5,000 | 7 | 4-5 | 35% |
| 10,000 | 15 | 8-10 | 40% |
| 50,000 | 70 | 35-45 | 45% |

### Message Optimization

```go
// Bandwidth reduction techniques

// 1. Delta compression - only send changed components
type EntityDelta struct {
    NetworkID    uint32
    ChangedMask  uint16           // Bitmask of changed components
    Components   []ComponentDelta // Only changed components
}

// 2. Quantization - reduce precision for network
type NetPosition struct {
    X int16  // Fixed point: actual = X / 10.0
    Y int16  // Saves 12 bytes vs float64
}

// 3. Batching - combine multiple updates
type TickUpdate struct {
    Tick      uint32
    Entities  []EntityDelta
    Events    []GameEvent
}
```

### Efficiency Optimization Tasks
- [ ] Reduce server tick rate to 20/sec (client stays 60 FPS)
- [ ] Implement hybrid tick rates (position: 20, state: 5, events: instant)
- [ ] Add sync.Pool for serialization buffers
- [ ] Pre-allocate component slices to avoid GC
- [ ] Implement delta compression (bitmask of changed components)
- [ ] Add position quantization (int16 fixed-point)
- [ ] Implement spatial interest management (view distance culling)
- [ ] Profile and optimize hot paths (pprof)

### Message Optimization Tasks
- [ ] Implement delta compression (only send changes)
- [ ] Use fixed-point integers for positions (int16 * 0.1)
- [ ] Batch all updates into single message per tick
- [ ] Compress large messages with snappy/lz4
- [ ] Track bandwidth per client for monitoring

### Graceful Degradation

```go
// server/core/health.go
type ServerHealth struct {
    CPUUsage        float64
    MemoryUsage     float64
    ActiveMatches   int
    ActivePlayers   int
    MessageRate     float64

    // Degradation thresholds
    HighLoad        bool  // > 70% CPU
    CriticalLoad    bool  // > 90% CPU
}

func (s *Server) checkHealth() {
    health := s.getHealth()

    if health.CriticalLoad {
        // Stop accepting new matches
        s.acceptingMatches = false
        // Reduce tick rate temporarily
        s.tickRate = 20
        // Alert monitoring
        s.metrics.AlertCritical()
    } else if health.HighLoad {
        // Reduce tick rate slightly
        s.tickRate = 25
        s.metrics.AlertWarning()
    } else {
        s.tickRate = 30
        s.acceptingMatches = true
    }
}
```

### Monitoring & Observability

```go
// server/metrics/prometheus.go
var (
    activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "doomerang_active_connections",
        Help: "Number of active WebSocket connections",
    })

    activeMatches = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "doomerang_active_matches",
        Help: "Number of active game matches",
    })

    messagesProcessed = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "doomerang_messages_processed_total",
        Help: "Total messages processed",
    })

    messageLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "doomerang_message_latency_seconds",
        Help:    "Message processing latency",
        Buckets: []float64{.001, .005, .01, .025, .05, .1},
    })

    rateLimitHits = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "doomerang_rate_limit_hits_total",
        Help: "Rate limit violations",
    }, []string{"type"})  // "connection", "message", "global"
)
```

### Monitoring Tasks
- [ ] Add Prometheus metrics endpoint `/metrics`
- [ ] Track connections, matches, message rates
- [ ] Track message processing latency histogram
- [ ] Track rate limit violations
- [ ] Add health check endpoint `/health`
- [ ] Set up Grafana dashboard (optional)
- [ ] Configure alerts for critical thresholds

### Deployment Configuration

```yaml
# docker-compose.yml
version: '3.8'
services:
  game-server:
    build: ./server
    ports:
      - "7373:7373"
    environment:
      - MAX_MATCHES=150
      - TICK_RATE=30
      - LOG_LEVEL=info
    deploy:
      resources:
        limits:
          memory: 1800M
          cpus: '1.5'
        reservations:
          memory: 512M
          cpus: '0.5'
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:7373/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

```ini
# systemd service (alternative to Docker)
# /etc/systemd/system/doomerang-server.service
[Unit]
Description=Doomerang Game Server
After=network.target

[Service]
Type=simple
User=gameserver
WorkingDirectory=/opt/doomerang
ExecStart=/opt/doomerang/doomerang-server
Restart=always
RestartSec=5
LimitNOFILE=65535

# Resource limits
MemoryMax=1800M
CPUQuota=150%

[Install]
WantedBy=multi-user.target
```

### Deployment Tasks
- [ ] Create Dockerfile for server
- [ ] Create docker-compose.yml
- [ ] Create systemd service file
- [ ] Configure NOFILE limits (ulimit)
- [ ] Set up log rotation
- [ ] Create deployment script

### Security Hardening

```go
// server/security/validation.go

// 1. Input validation
func ValidatePlayerInput(input *PlayerInput) error {
    // Sequence number sanity
    if input.Sequence > currentSeq + 100 {
        return ErrInvalidSequence
    }

    // Direction bounds
    if input.Direction < -1 || input.Direction > 1 {
        return ErrInvalidDirection
    }

    // Timestamp sanity (not too far in future/past)
    now := time.Now().UnixMilli()
    if input.Timestamp > now + 5000 || input.Timestamp < now - 30000 {
        return ErrInvalidTimestamp
    }

    return nil
}

// 2. Anti-cheat basics
func DetectSpeedHack(player *Player, input *PlayerInput) bool {
    // Check if player moved impossibly fast
    expectedMaxDistance := config.Player.MaxSpeed * tickDuration * 1.5
    actualDistance := distance(player.LastPos, player.Pos)
    return actualDistance > expectedMaxDistance
}
```

### Security Tasks
- [ ] Validate all incoming message fields
- [ ] Implement basic speed hack detection
- [ ] Add sequence number validation
- [ ] Sanitize player names (no script injection)
- [ ] Use secure WebSocket (wss://) in production
- [ ] Implement server-side game state validation

### Architecture Trade-offs

**Why Match-Based (not Single World)?**

| Approach | Pros | Cons | Max Players/VPS |
|----------|------|------|-----------------|
| Single World | Simpler, everyone together | CPU bottleneck, single point of failure | ~200 |
| Match-Based | Scalable, fault isolated | Need matchmaking | ~1000+ |
| Sharded World | Massive scale | Complex, zone transitions | ~5000 |

For PvP Doomerang, match-based is ideal because:
- Natural isolation (2-4 player fights)
- Horizontal scaling (add VPS = more capacity)
- No complex zone handoffs
- Matches can run on any server

**Why WebSocket (not UDP)?**

| Protocol | Latency | Browser Support | Reliability |
|----------|---------|-----------------|-------------|
| UDP | Best | No (WASM requirement fails) | Manual |
| WebSocket | Good | Yes | Built-in |
| WebRTC | Best | Yes (complex) | Manual |

WebSocket is the pragmatic choice for WASM support. If we dropped WASM:
- Could use UDP for 30-50% lower latency
- But lose browser play entirely

**Why 20-30 tick (not 60)?**

| Tick Rate | CPU Cost | Feel | Use Case |
|-----------|----------|------|----------|
| 60 | High | Twitch shooter | CS:GO, Valorant |
| 30 | Medium | Action game | Fortnite, Apex |
| 20 | Low | Platformer | Our choice |
| 10 | Minimal | Turn-based | Strategy games |

For a 2D platformer with client-side prediction, 20 tick feels identical to 60 tick because:
- Client renders at 60 FPS and interpolates
- Prediction hides input delay
- Visual smoothness comes from client, not server

### Master Server Infrastructure

The master server is a lightweight HTTP service separate from game servers:

```
┌─────────────────────────────────────────────────────────────┐
│                    Master Server                            │
│         (Single instance, ~$5/mo VPS or serverless)         │
├─────────────────────────────────────────────────────────────┤
│  Endpoints:                                                  │
│  - GET /servers         (client queries)                    │
│  - POST /servers/register (game server registers)           │
│  - POST /servers/heartbeat (game server keepalive)          │
│                                                              │
│  Storage: In-memory map with TTL (no database needed)       │
│  Scale: Handles 100K+ requests/sec (stateless, cacheable)   │
└─────────────────────────────────────────────────────────────┘
```

**Deployment Options:**
1. **Cheapest**: Single $5/mo VPS (DO, Vultr) - handles thousands of game servers
2. **Serverless**: Cloudflare Workers / AWS Lambda - pay per request, auto-scale
3. **Resilient**: Two VPS behind load balancer with shared Redis

**Master Server Implementation Tasks:**
- [ ] Create simple HTTP server (Go stdlib or chi router)
- [ ] In-memory server registry with sync.Map
- [ ] TTL-based cleanup goroutine (remove stale servers)
- [ ] Rate limiting (prevent registration spam)
- [ ] CORS headers for browser clients
- [ ] Optional: Redis backend for multi-instance deployment

### Scaling Strategy

**Phase 1: Single VPS (0-500 players)**
- Single game server instance
- Master server on same VPS or separate $5 VPS
- All features enabled
- Monitor and optimize

**Phase 2: Multiple VPS (500-5000 players)**
- 5-10 game server instances across regions
- Master server on dedicated small VPS
- Game servers auto-register with master on startup
- Clients see all servers, can filter by region

**Phase 3: Full Scale (5000+ players)**
- Dedicated matchmaking service
- Geographic server distribution (US, EU, Asia)
- Master server with Redis backend
- Database for persistent stats
- CDN for WASM client

### Scaling Tasks
- [ ] Design matchmaking protocol
- [ ] Implement server discovery/registration
- [ ] Add Redis for cross-server coordination (Phase 2)
- [ ] Implement geographic server selection (Phase 3)
- [ ] Add player statistics persistence (Phase 3)

---

## Production Implementation Checklist

### Rate Limiting & Protection
- [ ] Per-IP connection rate limiting (5/sec)
- [ ] Per-client message rate limiting (60/sec)
- [ ] Global circuit breaker (50K msg/sec)
- [ ] Temporary IP bans for violations
- [ ] Max connections per IP (4)
- [ ] Max message size validation (4KB)

### Resource Management
- [ ] Match-based isolation (not single world)
- [ ] 30 tick/sec server (not 60)
- [ ] Connection pooling and limits
- [ ] Goroutine budgeting
- [ ] Memory limits per match
- [ ] Automatic match cleanup

### Message Efficiency
- [ ] Delta compression (only changes)
- [ ] Position quantization (int16)
- [ ] Message batching
- [ ] Bandwidth monitoring

### Observability
- [ ] Prometheus metrics endpoint
- [ ] Health check endpoint
- [ ] Structured logging (JSON)
- [ ] Rate limit violation tracking
- [ ] Latency histograms

### Deployment
- [ ] Docker container
- [ ] systemd service
- [ ] NOFILE ulimit config
- [ ] Auto-restart on crash
- [ ] Log rotation

### Security
- [ ] Input validation
- [ ] Basic anti-cheat
- [ ] Secure WebSocket (wss://)
- [ ] Name sanitization

---

# Appendix A: Netcode Reference - Client-Side Prediction & Server Reconciliation

## Overview

This document describes the **Valve Source Engine style** netcode used in Doomerang multiplayer. This is NOT rollback netcode (GGPO). The key difference:

| Technique | Used In | How It Works |
|-----------|---------|--------------|
| **Server Reconciliation** | Half-Life, CS:GO, Fortnite, Apex | Server is authoritative. Client predicts local player, corrects on mismatch |
| **Rollback Netcode (GGPO)** | Fighting games (Street Fighter, MK) | All clients simulate. On desync, roll back and re-simulate |

We use **Server Reconciliation** because:
- Simpler to implement
- Doesn't require deterministic physics
- Better for variable player counts (2-4 players)
- Server remains authoritative (anti-cheat friendly)

---

## The Problem: Input Latency

Without prediction, a player pressing "jump" experiences:

```
[Client]                    [Server]                    [Client]
   |                           |                           |
   |------- Input: Jump ------>|                           |
   |        (50ms RTT)         |                           |
   |                           | Process input             |
   |                           | Update position           |
   |<----- New Position -------|                           |
   |        (50ms RTT)         |                           |
   |                           |                           |
   | Render new position       |                           |
   |                           |                           |
Total delay: 100ms (unplayable for action games)
```

---

## The Solution: Client-Side Prediction

The client **immediately** applies input locally, then reconciles when server state arrives:

```
[Client]                    [Server]
   |                           |
   | Input: Jump               |
   | Apply locally (instant!)  |
   |------- Input: Jump ------>|
   |                           | Process input
   |                           | Update position
   |<----- Server State -------|
   |                           |
   | Compare predicted vs      |
   | server position           |
   | Reconcile if needed       |
```

**Result**: Player sees immediate response. Corrections happen invisibly (if small) or with a small snap (if large).

---

## Algorithm: Server Reconciliation

### Step 1: Client Sends Input with Sequence Number

```go
type PlayerInput struct {
    Sequence  uint32    // Incrementing ID
    Actions   Actions   // Jump, MoveLeft, MoveRight, Attack
    Timestamp int64     // Client time
}

// Client sends input
func (c *Client) SendInput(input PlayerInput) {
    c.inputHistory[input.Sequence] = InputRecord{
        Input:     input,
        Predicted: c.localPlayer.Position, // Store predicted state
    }
    c.network.Send(input)
}
```

### Step 2: Client Applies Input Immediately (Prediction)

```go
func (c *Client) Update() {
    input := c.captureInput()
    input.Sequence = c.nextSequence
    c.nextSequence++

    // Apply locally BEFORE sending (prediction)
    c.applyInput(c.localPlayer, input)

    // Store for reconciliation
    c.inputHistory[input.Sequence] = InputRecord{
        Input:     input,
        Predicted: c.localPlayer.Position,
    }

    // Send to server
    c.SendInput(input)
}
```

### Step 3: Server Processes Input Authoritatively

```go
func (s *Server) ProcessInput(client *Client, input PlayerInput) {
    player := s.getPlayer(client)

    // Server applies input (authoritative)
    s.applyInput(player, input)

    // Track last processed sequence
    player.LastProcessedSequence = input.Sequence
}
```

### Step 4: Server Sends State with Last Acknowledged Sequence

```go
type PlayerState struct {
    NetworkID     uint32
    Position      Vector2
    Velocity      Vector2
    LastSequence  uint32  // Last input sequence processed
}
```

### Step 5: Client Reconciles

```go
func (c *Client) OnServerState(state PlayerState) {
    // Find our predicted state for this sequence
    predicted, exists := c.inputHistory[state.LastSequence]
    if !exists {
        return // Too old, ignore
    }

    // Calculate error
    error := distance(predicted.Predicted, state.Position)

    if error > c.snapThreshold {
        // Large error: snap to server position
        c.localPlayer.Position = state.Position
    } else if error > c.smoothThreshold {
        // Small error: interpolate toward server
        c.localPlayer.Position = lerp(c.localPlayer.Position, state.Position, 0.3)
    }
    // If error < smoothThreshold: no correction needed

    // Discard old history
    c.pruneHistory(state.LastSequence)

    // Re-apply unacknowledged inputs
    for seq := state.LastSequence + 1; seq < c.nextSequence; seq++ {
        if record, ok := c.inputHistory[seq]; ok {
            c.applyInput(c.localPlayer, record.Input)
        }
    }
}
```

---

## Key Insight: Input Replay

The most important part is **replaying unacknowledged inputs** after correction:

```
Timeline:
  Seq 1: Jump      [Server received, acknowledged]
  Seq 2: MoveRight [Server received, acknowledged]
  Seq 3: MoveRight [In flight...]
  Seq 4: Jump      [In flight...]
  Seq 5: MoveLeft  [Just sent]

When server state arrives acknowledging Seq 2:
  1. Snap/interpolate to server position at Seq 2
  2. Re-apply Seq 3, 4, 5 to get current predicted position
```

This ensures the client's predicted position is always:
`ServerPosition + Effects of Unacknowledged Inputs`

---

## Entity Interpolation (Remote Players)

For **other players**, we use interpolation, not prediction:

```go
func (c *Client) UpdateRemotePlayer(player *RemotePlayer) {
    // We receive positions at 20-30 Hz
    // We render at 60 Hz
    // Interpolate between known positions

    now := time.Now()
    renderTime := now.Add(-c.interpDelay) // Render 100ms in the past

    // Find two snapshots bracketing renderTime
    prev, next := player.FindSnapshots(renderTime)

    // Interpolate
    t := (renderTime - prev.Time) / (next.Time - prev.Time)
    player.RenderPosition = lerp(prev.Position, next.Position, t)
}
```

**Why 100ms delay?** To always have snapshots to interpolate between. Without delay, we'd have to extrapolate (guess) which causes visual errors.

---

## Configuration Values

```go
var Netcode = struct {
    // Prediction
    InputHistorySize   int     // 64 frames
    SnapThreshold      float64 // 50 pixels - snap if error > this
    SmoothThreshold    float64 // 5 pixels - ignore if error < this

    // Interpolation
    InterpDelay        time.Duration // 100ms - render delay for remotes
    InterpBufferSize   int           // 32 snapshots

    // Server
    TickRate           int     // 20-30 Hz
    MaxInputsPerTick   int     // 3 - prevent input flooding
}{
    InputHistorySize:  64,
    SnapThreshold:     50.0,
    SmoothThreshold:   5.0,
    InterpDelay:       100 * time.Millisecond,
    InterpBufferSize:  32,
    TickRate:          20,
    MaxInputsPerTick:  3,
}
```

---

## References

1. **Gabriel Gambetta - Fast-Paced Multiplayer** (Primary Reference)
   - [Client-Side Prediction and Server Reconciliation](https://www.gabrielgambetta.com/client-side-prediction-server-reconciliation.html)
   - [Entity Interpolation](https://www.gabrielgambetta.com/entity-interpolation.html)
   - [Lag Compensation](https://www.gabrielgambetta.com/lag-compensation.html)
   - [Live Demo](https://www.gabrielgambetta.com/client-side-prediction-live-demo.html)

2. **Valve Developer Wiki - Source Multiplayer Networking**
   - [Source Multiplayer Networking](https://developer.valvesoftware.com/wiki/Source_Multiplayer_Networking)
   - Describes cl_interp, cl_predict, and lag compensation

3. **Glenn Fiedler - Networked Physics**
   - [Networked Physics](https://gafferongames.com/post/networked_physics_2004/)
   - [State Synchronization](https://gafferongames.com/post/state_synchronization/)

4. **Wikipedia - Client-Side Prediction**
   - [Client-side prediction](https://en.wikipedia.org/wiki/Client-side_prediction)

---

## Comparison: Server Reconciliation vs Rollback

| Aspect | Server Reconciliation | Rollback (GGPO) |
|--------|----------------------|-----------------|
| Authority | Server only | All clients |
| Determinism | Not required | Required |
| Complexity | Moderate | High |
| Player count | Any | Usually 2 |
| Anti-cheat | Easy (server validates) | Hard (clients trusted) |
| Best for | FPS, action games | Fighting games |
| Examples | CS:GO, Fortnite | Street Fighter V, MK11 |

**Why we chose Server Reconciliation:**
- Doomerang has 2-4 players (not just 2)
- Server can prevent cheating
- Physics doesn't need to be deterministic
- Simpler to implement with necs/donburi

---

## Implementation Checklist

### Client-Side Prediction
- [ ] Add sequence number to PlayerInput
- [ ] Store input history ring buffer (64 frames)
- [ ] Apply input locally before sending
- [ ] Store predicted position with each input
- [ ] On server state: compare and reconcile
- [ ] Replay unacknowledged inputs after correction

### Entity Interpolation
- [ ] Store position history for remote players (32 frames)
- [ ] Calculate render time (now - 100ms)
- [ ] Find bracketing snapshots
- [ ] Lerp between snapshots for rendering

### Tuning
- [ ] Adjust snap threshold (start: 50px)
- [ ] Adjust smooth threshold (start: 5px)
- [ ] Adjust interp delay (start: 100ms)
- [ ] Profile and optimize reconciliation
