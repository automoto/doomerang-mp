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
/components        Data-only structures
/systems           Game logic (Update functions)
/systems/factory   Entity creation functions
/archetypes        Pre-defined component bundles
/tags              Entity and collision tags
/config            Global constants and configuration
/scenes            Game state management
/assets            Tiled maps, spritesheets, audio
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
    ActiveBoomerang     *donburi.Entry // Currently thrown boomerang
    // ...
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
    Timer          int            // Match timer (frames remaining)
    Scores         []PlayerScore  // Per-player statistics
    WinnerIndex    int            // Winner (-1 = no winner, -2 = tie)
    WinningTeam    int            // For team modes
}

type PlayerScore struct {
    PlayerIndex int
    KOs         int  // Kills/knockouts
    Deaths      int
    Team        int  // 0 or 1 for team modes, -1 for FFA
}
```

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

Input is decoupled from raw polling via `InputData`:

```go
type InputData struct {
    Current  [cfg.ActionCount]bool  // This frame
    Previous [cfg.ActionCount]bool  // Last frame
}

// Usage in systems:
if systems.GetAction(input, cfg.ActionJump).JustPressed { ... }
```

**Never** call `ebiten.IsKeyPressed()` directly in game systems.

---

## System Execution Order

Systems run in a specific order each frame:

1. **UpdateInput** - Polls raw input, populates InputData
2. **UpdateMatch** - Match timer, state transitions
3. **UpdatePlayer** - Player state machine, movement intent
4. **UpdateBot** - AI decision making (generates input)
5. **UpdateCombatHitboxes** - Creates/updates attack hitboxes
6. **UpdateBoomerang** - Projectile physics and collision
7. **UpdateCombat** - Processes DamageEvents, applies damage
8. **UpdatePhysics** - Movement, gravity, collision resolution
9. **UpdateDeath** - Death timers, respawn logic
10. **UpdateCamera** - Follow players, dynamic zoom

**Critical**: `UpdateCombat` must run after hitbox/boomerang systems so `DamageEvent` components exist when processed.

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
