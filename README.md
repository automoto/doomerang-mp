# Doomerang

Doomerang is a fast-paced **local multiplayer fighting game** built with **Go** and the [Ebitengine](https://ebitengine.org/) game engine. Up to 4 players battle it out using melee attacks and boomerangs in arena-style combat.

<img width="534" height="321" alt="Screenshot 2026-01-28 at 3 33 02 AM" src="https://github.com/user-attachments/assets/94d9e5d8-c741-46b0-98f2-66abde1cfa26" />

## Features

- **Local Multiplayer**: 1-4 players on the same machine
- **Multiple Game Modes**: Free-for-all, 1v1, 2v2 team battles, Co-op vs Bots
- **Combat System**: Punches, kicks, jump kicks, and throwable boomerangs
- **Bot AI**: Play against AI opponents with configurable difficulty
- **Dynamic Camera**: Automatically zooms and pans to keep all players in view

## Quick Start

### Prerequisites
- [Go 1.21+](https://go.dev/dl/)
- OS-specific dependencies (see [Ebitengine installation](https://ebitengine.org/en/documents/install.html))

### Run
```bash
make run
```

### Controls

| Action | Player 1 (Keyboard A) | Player 2 (Keyboard B) | Gamepad |
|--------|----------------------|----------------------|---------|
| Move | Arrow Keys | WASD | Left Stick / D-Pad |
| Jump | Numpad 0 | Space | A (Xbox) / Cross (PS) |
| Attack | Numpad 1 | Q | X (Xbox) / Square (PS) |
| Boomerang | Numpad 2 | E | B (Xbox) / Circle (PS) |

## Architecture

The project follows an ECS (Entity Component System) pattern using the [donburi](https://github.com/yohamta/donburi) library:

```
/components    Data-only structures (PlayerData, MatchData, BoomerangData)
/systems       Game logic (UpdateCombat, UpdateBoomerang, UpdateMatch)
/systems/factory  Entity creation using archetypes
/scenes        Game state management (Menu, Lobby, World)
/assets        Tiled maps, spritesheets, and audio
/config        Global constants, states, input bindings, game modes
/archetypes    Pre-defined component bundles for entity types
/tags          Entity and collision tags for filtering
```

For a deeper dive into the ECS implementation, see [ECS_AND_DONBURI.md](docs/ECS_AND_DONBURI.md).

### Technical Highlights

- **PvP Combat System**: Unified hit detection supporting player-vs-player, player-vs-enemy, and team-based friendly fire prevention
- **Match System**: Configurable game modes with KO tracking, team scores, and win conditions
- **Zero-Allocation Rendering**: Reuses drawing options to eliminate per-frame GC pressure
- **Asset Caching**: Centralized system prevents redundant asset decoding
- **Memory Safety**: Uses pointer wrappers (`ObjectData`) for stable physics references during ECS reallocation
- **Type-Safe States**: Centralized `StateID` enum prevents typo-related bugs
- **Input Abstraction**: Supports multiple input devices per player with action mapping

> **macOS Note**: The Makefile uses `CGO_CFLAGS="-w"` to suppress Metal backend warnings.

## Game Modes

| Mode | Description |
|------|-------------|
| Free-for-All | Every player for themselves, most KOs wins |
| 1v1 | Two players, most KOs wins |
| 2v2 | Team battle, combined team KOs wins |
| Co-op vs Bots | All humans vs AI opponents |

## Assets

Assets are a mix of sourced and original creations:
- [Fire Effects](https://stealthix.itch.io/animated-shots)
- [Other Effects](https://stealthix.itch.io/animated-effects)
- [Character](https://deadrevolver.itch.io/pixel-prototype-player-sprites)
- [Tiles](https://overcrafted.itch.io/cyberpunk-platformer-tileset)
- [Music](https://davidkbd.itch.io/interstellar-edm-metal-music-pack)
- [Sound Effects](https://heltonyan.itch.io/pixelcombat)
