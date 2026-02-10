# Entity Component System (ECS) and the `donburi` Library

This document explains the ECS architecture and multiplayer systems implemented in Doomerang using the `donburi` library.

## What is ECS?

ECS is a software architectural pattern primarily used in game development. It follows the principle of "composition over inheritance," which helps to create flexible and efficient game logic. The architecture is built on three core concepts:

- **Entities**: Unique identifiers for game objects. An entity is essentially a number with no data or behavior.
- **Components**: Plain data structures that hold the state of an entity. They contain no logic.
- **Systems**: Functions that implement game logic by operating on entities with specific components.

### Why Use ECS?

- **Flexibility**: Entities are defined by their components, making it easy to create new types by mixing and matching.
- **Performance**: Components are stored contiguously in memory, enabling cache-friendly iteration.
- **Decoupling**: Separation of data (components) from logic (systems) leads to a maintainable codebase.

---

## Project Structure

```
/components          Data-only structures (local game components)
/systems             Game logic (Update functions)
/systems/factory     Entity creation functions
/archetypes          Pre-defined component bundles
/tags                Entity and collision tags
/config              Global constants and configuration
/scenes              Game scenes (menu, lobby, world, networked, serverbrowser)
/assets              Tiled maps, spritesheets, audio, shaders
/ui                  ebitenui widget layouts
/network             Client-side networking (WebSocket client, prediction buffer)
/server              Dedicated game server (core/, cmd/)
/shared              Code shared between client and server
/shared/messages     Network message types
/shared/netcomponents Network-synced ECS components
/shared/netconfig    Shared enums (ActionID, StateID for network)
/shared/leveldata    TMX level parser (used by both client and server)
/shared/protocol     necs component registration
/fonts               Font management
/mathutil            Math utilities
```

---

## Core Components

### Player Component

The `PlayerData` component identifies players in multiplayer:

```go
type PlayerData struct {
    PlayerIndex         int            // 0-3 player index for multiplayer
    Direction           Vector         // Facing direction
    ComboCounter        int            // Punch/kick sequences
    InvulnFrames        int            // Invulnerability timer
    BoomerangChargeTime int            // Charge time for boomerang throw
    ActiveBoomerang     *donburi.Entry // Currently thrown boomerang
    ChargeVFX           *donburi.Entry // VFX shown while charging boomerang
    LastSafeX           float64        // Last grounded position (for respawn)
    LastSafeY           float64
    OriginalSpawnX      float64        // Spawn point assigned at match start
    OriginalSpawnY      float64
}
```

**Key Field**: `PlayerIndex` is critical for:
- Preventing self-damage (player can't hit themselves)
- Team identification (same team = no friendly fire)
- KO attribution (tracking who killed whom)

### Match Component

The `MatchData` component is a singleton that tracks the current match state:

```go
type MatchData struct {
    State          MatchStateID   // Countdown, Playing, Finished
    GameMode       GameModeID     // FFA, 1v1, 2v2, CoopVsBots
    Timer          int            // Countdown or match timer (frames remaining)
    Duration       int            // Total match duration (frames)
    Scores         []PlayerScore  // Per-player statistics
    WinnerIndex    int            // Winner (-1 = no winner, -2 = tie)
    WinningTeam    int            // For team modes (-1 if N/A)
    CountdownValue int            // Current countdown number (3, 2, 1, GO)
}

type PlayerScore struct {
    PlayerIndex int
    KOs         int  // Kills/knockouts
    Deaths      int
    Team        int  // 0 or 1 for team modes, -1 for FFA
}
```

Methods: `GetPlayerScore()`, `AddKO()`, `AddDeath()`, `GetLeader()`, `GetTeamScore()`

### Damage Event Component

Damage is applied through a queued event system:

```go
type DamageEventData struct {
    Amount        int     // Damage to apply
    KnockbackX    float64 // Optional knockback
    KnockbackY    float64
    AttackerIndex int     // PlayerIndex of attacker for KO tracking
}
```

**Important**: `DamageEvent` is a transient component. Add it to apply damage, and the `UpdateCombat` system processes and removes it.

---

## Multiplayer Combat System

### Architecture Overview

The combat system supports:
- **PvP**: Players can damage other players
- **PvE**: Players can damage enemies (bots are players with Bot component)
- **Team-based**: Teammates cannot damage each other

### Hit Detection Flow

```
1. Player attacks (punch/kick/boomerang)
       ↓
2. Hitbox created with OwnerEntity reference
       ↓
3. checkHitboxCollisions() runs:
   - Get owner's PlayerIndex
   - Check collision with ResolvPlayer and ResolvEnemy tags
   - Skip self (same PlayerIndex)
   - Skip teammates (areTeammates check)
   - Call applyHitToTarget() for valid hits
       ↓
4. applyHitToTarget() triggers:
   - Visual effects (flash, particles, screen shake)
   - Adds DamageEvent component
   - Applies knockback
       ↓
5. UpdateCombat() processes DamageEvent:
   - Checks invulnerability (skips if invuln)
   - Applies damage to Health.Current
   - Sets player state to Stunned
   - Sets InvulnFrames (prevents stun-lock)
   - Removes DamageEvent component
       ↓
6. If Health.Current == 0:
   - Death sequence triggers
   - KO credited to AttackerIndex
```

### Team-Based Friendly Fire Prevention

The `areTeammates()` function prevents friendly fire:

```go
func areTeammates(ecs *ecs.ECS, playerIndex1, playerIndex2 int) bool {
    match := components.Match.Get(matchEntry)

    score1 := match.GetPlayerScore(playerIndex1)
    score2 := match.GetPlayerScore(playerIndex2)

    // Team -1 means FFA (no team), so not teammates
    if score1.Team == -1 || score2.Team == -1 {
        return false
    }

    return score1.Team == score2.Team
}
```

This is checked in:
- `checkHitboxCollisions()` for melee attacks
- `checkCollisions()` in boomerang.go for projectiles

### Bots Are Players

Bots use `PlayerData` with `PlayerIndex`, not a separate enemy type:

```go
func CreateBotPlayer(ecs *ecs.ECS, x, y float64, playerIndex int, difficulty BotDifficulty) {
    player := CreatePlayer(ecs, x, y, inputCfg)  // Same as human player
    player.AddComponent(components.Bot)           // Add AI component
}
```

This means bots automatically work with the PvP system - they can hit and be hit by all players.

---

## Component Patterns

### Pointer Wrapper Pattern

External library objects (like `resolv.Object`) must be wrapped:

```go
type ObjectData struct {
    *resolv.Object
}
var Object = donburi.NewComponentType[ObjectData]()
```

**Why?** Donburi stores components in slices that reallocate as they grow. The wrapper ensures the pointer held by external libraries remains valid.

### Reverse Lookup Pattern

Always set `obj.Data` for O(1) entity lookup from physics objects:

```go
obj := resolv.NewObject(x, y, w, h)
obj.Data = entry  // Critical for collision callbacks
```

### State Management

Characters use a type-safe `StateID` enum:

```go
type StateData struct {
    CurrentState  config.StateID
    PreviousState config.StateID
    StateTimer    int
}
```

States are defined in `config/states.go`. Never use string comparisons for state.

### Input Abstraction

Input is decoupled from raw polling via `InputData` (global) and `PlayerInputData` (per-player):

```go
type InputData struct {
    Current         [cfg.ActionCount]bool  // This frame
    Previous        [cfg.ActionCount]bool  // Last frame
    LastInputMethod InputMethod            // Keyboard, Xbox, PlayStation
}

type PlayerInputData struct {
    PlayerIndex    int                     // 0-3 player index
    CurrentInput   [cfg.ActionCount]bool
    PreviousInput  [cfg.ActionCount]bool
    BoundGamepadID *ebiten.GamepadID       // Bound gamepad (nil = keyboard)
    ControlScheme  cfg.ControlSchemeID     // Scheme A or B
    InputMethod    InputMethod
}

// Usage in systems:
if systems.GetAction(input, cfg.ActionJump).JustPressed { ... }
```

**Never** call `ebiten.IsKeyPressed()` directly in game systems.

**Exception**: `systems/netinput.go` polls input directly because it sends raw input to the server as `PlayerInput` messages, bypassing the local ECS input pipeline.

---

## System Execution Order

### Local Game (scenes/world.go)

Systems run in a specific order each frame:

```
Always run:
 1. UpdateAudio           - Music and sound effects
 2. UpdateInput           - Polls raw input, populates InputData
 3. UpdateBots            - AI decision making (generates input)
 4. UpdateMultiPlayerInput - Merges per-player input from gamepads/keyboard
 5. UpdatePause           - Pause menu handling

Gameplay systems (paused when game is paused):
 6. UpdatePlayer          - Player state machine, movement intent
 7. UpdateEnemies         - Enemy behavior
 8. UpdateStates          - State transitions
 9. UpdatePhysics         - Movement, gravity
10. UpdateCollisions      - Collision detection and resolution
11. UpdateObjects         - Physics object updates
12. UpdateBoomerang       - Projectile physics and collision
13. UpdateKnives          - Knife projectiles
14. UpdateCombat          - Processes DamageEvents, applies damage
15. UpdateCombatHitboxes  - Creates/updates attack hitboxes
16. UpdateDeaths          - Death timers, respawn logic
17. UpdateFire            - Fire hazard system
18. UpdateEffects         - Visual effects
19. UpdateMessage         - On-screen messages

Always run:
20. UpdateMatch           - Match timer, state transitions
21. UpdateSettings        - Settings state
22. UpdateSettingsMenu    - Settings menu
23. UpdateCamera          - Follow players, dynamic zoom
```

### Networked Game (scenes/networked.go)

Networked scenes use a different, simpler system set:

```
1. NewNetworkInputSystem      - Polls input, sends to server, applies local prediction
2. NewNetInterpSystem         - Interpolates/extrapolates remote player positions
3. UpdateNetAnimations        - Advances animation frames based on NetPlayerState.StateID
4. NewNetPlayerEffectsSystem  - Detects jump/land transitions → SFX, dust VFX, squash/stretch
5. NewNetCameraSystem         - Follows local player via NetPosition
6. NewNetBoomerangEventSystem - Handles boomerang throw/catch/hit events
7. UpdateEffects              - Animates VFX (dust, particles, squash/stretch decay)
8. UpdateAudio                - Music and sound effects
```

Server snapshots are applied before systems run via `applySnapshot()`.

### Offline/Online Code Sharing

The offline (`systems/player.go`) and online (`systems/netinput.go` + `server/core/physics.go`) systems implement overlapping logic. To prevent divergence bugs, share logic wherever possible:

- **Pure physics helpers** belong in `shared/gamemath/` (e.g., `ApplyFriction()`, `ClampSpeed()`, `GetSlopeSurfaceY()`). Both offline, server, and prediction physics use these
- **Effects triggers** (SFX, VFX, squash/stretch) should be reusable helpers, not duplicated per scene. `triggerJumpEffects()` and `triggerLandEffects()` in `netplayereffects.go` demonstrate this pattern
- **State derivation** — converting physics state to animation state (Idle/Running/Jump) — should converge to a single shared function used by both `deriveState()` on the server and `applyPrediction()` on the client
- **Server physics changes must be mirrored in client prediction** (`systems/netprediction.go`) and vice versa. Both use the same config values from `config/config.go`

---

## Factory Pattern

Factories own the entire entity creation process:

```go
// GOOD - Factory creates everything
func CreatePlayer(ecs *ecs.ECS, x, y float64, inputCfg PlayerInputConfig) *donburi.Entry {
    player := archetypes.Player.Spawn(ecs)

    obj := resolv.NewObject(x, y, w, h)
    obj.Data = player  // Reverse lookup
    obj.AddTags(tags.ResolvPlayer)

    components.Player.SetValue(player, components.PlayerData{
        PlayerIndex: inputCfg.PlayerIndex,
        // ...
    })

    return player
}

// BAD - Caller creates objects
func CreatePlayer(ecs *ecs.ECS, obj *resolv.Object) *donburi.Entry { ... }
```

---

## Configuration Management

All tuning values live in `config/config.go`:

```go
var Combat = CombatConfig{
    PlayerPunchDamage:    22,
    PlayerInvulnFrames:   45,
    KnockbackUpwardForce: -4.0,
    // ...
}
```

**Never** hardcode values in systems. Always reference `config.*`.

---

## Best Practices Checklist

### Performance

1. **Zero-Allocation Rendering**: Reuse `ebiten.DrawImageOptions` and `color.RGBA` - don't create new ones each frame.

2. **Component Caching**: Call `Get()` once per entity in loops:
   ```go
   components.Player.Each(ecs.World, func(e *donburi.Entry) {
       player := components.Player.Get(e)  // Cache once
       physics := components.Physics.Get(e)
       // Use player and physics multiple times
   })
   ```

3. **HasComponent Caching**: In hot loops, cache the bool:
   ```go
   isPlayer := e.HasComponent(components.Player)
   isEnemy := e.HasComponent(components.Enemy)
   // Use bools instead of repeated HasComponent calls
   ```

4. **State Change Detection**: Only update tags when state actually changes:
   ```go
   if state.CurrentState == state.PreviousState { return }
   // ... update tags ...
   state.PreviousState = state.CurrentState
   ```

### Safety

5. **Pointer Wrappers**: Always use `ObjectData` for external library objects.

6. **Factory Integrity**: Factories must set `obj.Data = entry` and correct tags.

7. **Config Centralization**: All magic numbers belong in `config/config.go`.

8. **State Safety**: Add new states to `config/states.go` as `StateID` constants.

9. **Input Abstraction**: Never poll input directly in game systems.

### Multiplayer

10. **Use PlayerIndex**: For self-hit prevention, use `PlayerIndex` comparison, not entity comparison:
    ```go
    // GOOD
    if targetPlayerIndex == ownerPlayerIndex { continue }

    // BAD - May fail for projectiles
    if targetEntry == ownerEntry { continue }
    ```

11. **Team Checks**: Always check `areTeammates()` before applying PvP damage.

12. **DamageEvent Flow**: Don't set `InvulnFrames` before adding `DamageEvent` - the combat system handles invulnerability after processing damage.

13. **AttackerIndex Tracking**: Always set `AttackerIndex` in `DamageEvent` for proper KO attribution.

---

## Common Pitfalls

### Invulnerability Bug

**Wrong**: Setting invuln frames before damage event is processed
```go
player.InvulnFrames = 45  // BAD - combat.go will skip the damage
donburi.Add(e, components.DamageEvent, &DamageEventData{...})
```

**Right**: Let `UpdateCombat` set invuln frames after processing
```go
donburi.Add(e, components.DamageEvent, &DamageEventData{...})
// combat.go sets InvulnFrames after applying damage
```

### Missing Reverse Lookup

**Wrong**: Forgetting to set `obj.Data`
```go
obj := resolv.NewObject(x, y, w, h)
// obj.Data not set - collision callbacks can't find the entity!
```

**Right**: Always link physics object to entity
```go
obj := resolv.NewObject(x, y, w, h)
obj.Data = entry
```

### Direct Input Polling

**Wrong**: Polling keys in game logic
```go
if ebiten.IsKeyPressed(ebiten.KeyX) { jump() }
```

**Right**: Reading from InputData
```go
if systems.GetAction(input, cfg.ActionJump).Pressed { jump() }
```

### Removing Entities During Iteration

**Wrong**: Removing entities inside `Query.Each()` — donburi uses swap-remove in archetype storage, which skips the entity swapped into the removed slot:
```go
query.Each(world, func(entry *donburi.Entry) {
    if shouldRemove(entry) {
        world.Remove(entry.Entity()) // BAD - skips swapped entity
    }
})
```

**Right**: Collect first, then remove in a separate pass:
```go
var toRemove []donburi.Entity
query.Each(world, func(entry *donburi.Entry) {
    if shouldRemove(entry) {
        toRemove = append(toRemove, entry.Entity())
    }
})
for _, entity := range toRemove {
    world.Remove(entity)
}
```

---

## Network Multiplayer Architecture

The game supports network multiplayer via a server-authoritative model using [necs](https://github.com/leap-fish/necs) for WebSocket transport and entity synchronization.

### Architecture Overview

```
┌──────────────┐                         ┌──────────────┐
│    Client     │                         │    Server     │
│              │                         │              │
│ Poll input   │── PlayerInput ─────────▶│ Command queue │
│ Predict local│                         │ Run physics   │
│ Interpolate  │◀── WorldSnapshot ───────│ DoSync()     │
│ Render       │                         │              │
└──────────────┘                         └──────────────┘
```

### Network Components (`shared/netcomponents/`)

Components synced via necs between server and clients:

- **NetPosition** — Authoritative X/Y position
- **NetVelocity** — Velocity (used for extrapolation and reconciliation)
- **NetPlayerState** — StateID (Idle/Running/Jump), Direction, Health, LastSequence, IsLocal flag

### Client-Side Prediction

The local player sees instant movement via client-side prediction (`systems/netprediction.go`):
1. Client applies input locally with same physics as server (gravity, collision, slopes)
2. Server snapshots are reconciled using position smoothing (not seq-based replay)
3. Small errors: gentle correction per tick. Large errors (>20px): hard snap (teleport/respawn)

### Entity Sync Lifecycle

1. Server creates entity with net components on player join
2. `srvsync.NetworkSync()` registers entity for sync
3. Client receives `WorldSnapshot`, creates local entity with matching net components
4. Client adds `Animation`, `NetInterp`, and `SquashStretch` components for rendering and effects
5. Remote entities are interpolated between snapshots; extrapolated when client frames outpace server ticks
6. `NetPlayerEffectsSystem` detects state transitions and triggers SFX/VFX for all players
7. Stale entities collected and removed in a separate pass (avoids swap-remove issue)

### Reconciliation Patterns

- **Position smoothing**: Small errors get gentle per-tick correction; large errors (>snap threshold) hard-snap
- **Locked state gating**: Server-locked animation states (Throw, Hit) are preserved by client prediction (`applyPrediction()` skips them). `reconcileLocal()` uses `animStillPlaying()` to let animations complete before accepting server transitions
- **Velocity guard for transitions**: Jump detection uses `VelY < 0` to filter false positives caused by reconciliation setting `OnGround = true` at jump apex

### Key necs API

```go
// Server
srvsync.UseEsync(world)       // Initialize entity sync for world
srvsync.NetworkSync()          // Register entity for synchronization
srvsync.DoSync()               // Broadcast WorldSnapshot to all clients

// Client
esync.FindByNetworkId(world, id) // Find entity by network ID (returns donburi.Null if not found)
esync.Mapper.Deserialize(bytes)  // Deserialize component data from snapshot

// Shared
router.On[T]()                 // Register message handler (auto-registers type)
router.OnConnect               // Connection lifecycle callback
router.OnDisconnect            // Disconnection lifecycle callback
router.ResetRouter()           // Clear all handlers (needed between reconnects)
```

**Important**: necs callbacks run in goroutines. Use mutexes or command channels to safely access game state from callbacks.
