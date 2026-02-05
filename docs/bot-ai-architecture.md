# Bot AI Architecture

This document describes the bot AI system in `systems/bot.go`.

## Overview

Bots are player entities with an additional `Bot` component storing AI state. They generate input like human players—the same `UpdatePlayer` system handles both.

```
UpdateInput() → UpdateBots() → UpdateMultiPlayerInput() → UpdatePlayer()
                    ↑
              AI generates input here
```

## State Machine

The bot uses four states:

| State | Purpose | Transitions |
|-------|---------|-------------|
| **IDLE** | Patrol when no target found | → CHASE on target acquired |
| **CHASE** | Pursue target | → ATTACK in range, → RETREAT if low health |
| **ATTACK** | Engage in combat | → CHASE if target escapes, → RETREAT if low health |
| **RETREAT** | Evasive combat at low health | → CHASE when health recovers |

```
              IDLE (patrol)
                  │
                  ↓ target found
    ┌─────────────────────────────┐
    │  CHASE ←───────→ ATTACK    │
    │    ↑                 │     │
    │    └─── RETREAT ←────┘     │
    │         (low health)       │
    └─────────────────────────────┘
```

## Per-Frame Decision Flow

Each frame, `updateBotAI()` executes:

1. **Cooldown management** — Decrement timers (decision, attack, jump)
2. **Clear input** — Reset all actions to false
3. **Acquire targets** — Find nearest enemy and nearby teammates
4. **Threat check** — Scan for incoming projectiles; if found, generate defensive input and return early
5. **State update** — Evaluate health/distance to pick state
6. **Input generation** — Call state-specific handler

Threat detection has priority—bots dodge projectiles even mid-attack.

## Team System

Team values: `-1` (FFA), `0` (Team A), `1` (Team B)

Team-aware behaviors:
- **Target selection** — Skip teammates
- **Threat detection** — Ignore teammate projectiles
- **Collision avoidance** — Navigate around teammates

Team info flows from `MatchData.Scores[playerIndex].Team` to bot decisions.

## Teammate Collision Avoidance

Bots detect and navigate around teammates to prevent gridlock:

1. **Detection** — Find teammates within 80px
2. **Blocking check** — Is teammate within 50px horizontally and 40px vertically?
3. **Response** — Jump over if on ground, continue moving if airborne

All states (IDLE, CHASE, ATTACK, RETREAT) include teammate avoidance.

## Navigation

### Gap Detection

Bots check ahead for gaps and jump if the gap is within jump range (< 240px):

1. Check 24px ahead of bot
2. Look 48px down for ground
3. No ground found → measure gap width
4. Gap jumpable → trigger jump

### Line of Sight

Before throwing projectiles, bots raycast to the target. If any check hits a solid object, the bot won't throw.

### Pathfinding

A* navigation grid (32px cells) is created once per level and cached. Includes jump connections with higher cost for vertical movement.

## Combat

### Attack Selection

Bots use weighted random selection based on distance:

| Attack | Range | Weight | Notes |
|--------|-------|--------|-------|
| Punch | < 60px | 5.0 | Primary melee |
| Jump Kick | 50-100px | 0.5 | Rare, flashy |
| Boomerang | 80-300px | 2.0 | Requires LOS |
| Approach | > 50px | 3.0 | Close distance |

Weighted randomness creates varied, unpredictable behavior.

### Retreat Behavior

Low-health bots fight aggressively while evading:

| Distance | Movement | Attacks |
|----------|----------|---------|
| < 60px | Back away, panic jumps | Punch |
| 60-150px | Strafe pattern (L/pause/R/approach) | Jump kick or punch |
| > 150px | Chase enemy | Boomerang |

Additional: 2% chance per frame for random evasive jump.

## Threat Detection

Bots detect incoming projectiles using:
- Distance < 200px
- Time to hit < 60 frames
- Dot product confirms projectile heading toward bot

Responses by trajectory height:
- **High** — Duck
- **Mid** — Sidestep
- **Low** — Jump

Only projectiles trigger defensive responses; melee is too fast to react to.

## Configuration

All tuning values in `config/config.go`:

### Pathfinding
```go
CellSize:                  32.0   // Nav grid resolution
MaxJumpHeight:            150.0   // v²/(2g)
MaxJumpDistance:          240.0   // speed × airTime
GapCheckDist:              24.0   // Lookahead distance
GapCheckDepth:             48.0   // Ground check depth
TeammateDetectRange:       80.0   // Teammate scan range
TeammateBlockingDist:      50.0   // Blocking threshold
TeammateVerticalTolerance: 40.0   // Same-platform tolerance
```

### Combat
```go
PunchRange:        60.0
JumpKickMinRange:  50.0
JumpKickMaxRange: 100.0
BoomerangMinRange: 80.0
BoomerangMaxRange: 300.0
```

### Difficulty
```go
Easy:   { ReactionDelay: 30, AttackRange: 50.0, RetreatThreshold: 0.2 }
Normal: { ReactionDelay: 15, AttackRange: 60.0, RetreatThreshold: 0.3 }
Hard:   { ReactionDelay:  5, AttackRange: 70.0, RetreatThreshold: 0.15 }
```

## Performance

Pre-allocated structures avoid per-frame allocations:

```go
var attackOptions  = make([]AttackChoice, 0, 4)
var attackWeights  = make([]float64, 0, 4)
var cachedThreatInfo ThreatInfo
var cachedTeammates  = make([]*playerInfo, 0, 4)
var cachedNavGrid    *NavGrid
```

Teammate slice stores pointers and resets via `slice[:0]` each frame.

## Debugging

| Issue | Check |
|-------|-------|
| Bot won't move | `IsMatchPlaying()`, target acquisition, cooldowns |
| Excessive jumping | `JumpCooldown` values, gap detection, teammate blocking |
| Ignores threats | Distance < 200px, time to hit < 60, `ReactionDelay` |
| Stuck on teammates | Same team check, `TeammateDetectRange`, `TeammateBlockingDist` |

## Adding New Attacks

1. Add to `AttackChoice` enum
2. Add case in `chooseAttack()` with distance/weight
3. Handle in `generateAttackInputs()` switch
4. Add config values to `BotCombatConfig`
