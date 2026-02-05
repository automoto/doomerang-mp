package systems

import (
	"github.com/automoto/doomerang-mp/assets"
	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/tags"
	"github.com/solarlune/resolv"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

func UpdateDeaths(ecs *ecs.ECS) {
	components.Death.Each(ecs.World, func(e *donburi.Entry) {
		death := components.Death.Get(e)
		death.Timer--
		if death.Timer <= 0 {
			// Handle player death differently - respawn or game over
			if e.HasComponent(tags.Player) {
				handlePlayerDeath(ecs, e)
				return
			}

			// Non-player entity: remove from world
			spaceEntry, _ := components.Space.First(e.World)
			space := components.Space.Get(spaceEntry)
			if obj := components.Object.Get(e); obj != nil {
				space.Remove(obj.Object)
			}
			ecs.World.Remove(e.Entity())
		}
	})
}

// Game over delay in frames (30 frames = 0.5 seconds at 60fps)
const gameOverDelayFrames = 30

func handlePlayerDeath(ecs *ecs.ECS, e *donburi.Entry) {
	lives := components.Lives.Get(e)
	death := components.Death.Get(e)
	playerData := components.Player.Get(e)

	// Track KO/death in match scores
	trackMatchScore(ecs, playerData.PlayerIndex, death.LastAttackerIndex)

	// Game over delay expired - remove player
	if lives.Lives <= 0 {
		spaceEntry, _ := components.Space.First(e.World)
		space := components.Space.Get(spaceEntry)
		if obj := components.Object.Get(e); obj != nil {
			space.Remove(obj.Object)
		}
		ecs.World.Remove(e.Entity())
		return
	}

	// Death zone already decremented lives at collision time
	if !death.IsDeathZone {
		lives.Lives--
	}

	if lives.Lives <= 0 {
		// Last life lost - delay before game over
		death.Timer = gameOverDelayFrames
		return
	}

	donburi.Remove[components.DeathData](e, components.Death)
	RespawnPlayerNearDeath(ecs, e)
}

// trackMatchScore updates match scores when a player dies
func trackMatchScore(ecs *ecs.ECS, victimIndex, attackerIndex int) {
	matchEntry, ok := components.Match.First(ecs.World)
	if !ok {
		return
	}
	match := components.Match.Get(matchEntry)

	// Record death for victim
	match.AddDeath(victimIndex)

	// Award KO to attacker (if not self-kill or environment)
	if attackerIndex >= 0 && attackerIndex != victimIndex {
		match.AddKO(attackerIndex)
	}
}

// RespawnPlayer resets the player to default spawn with full health and lives.
func RespawnPlayer(ecs *ecs.ECS, e *donburi.Entry) {
	levelEntry, ok := components.Level.First(ecs.World)
	if !ok {
		return
	}
	levelData := components.Level.Get(levelEntry)

	if len(levelData.CurrentLevel.PlayerSpawns) == 0 {
		return
	}
	spawn := levelData.CurrentLevel.PlayerSpawns[0]

	resetPlayerAtPosition(e, spawn.X, spawn.Y)

	lives := components.Lives.Get(e)
	lives.Lives = lives.MaxLives

	// Reset message state so messages can be shown again
	ResetMessageState(ecs)
}

// RespawnPlayerNearDeath respawns the player at the best available spawn point.
// It considers spawn points, distance from enemies, and the death location.
func RespawnPlayerNearDeath(ecs *ecs.ECS, e *donburi.Entry) {
	spaceEntry, ok := components.Space.First(ecs.World)
	if !ok {
		RespawnPlayer(ecs, e)
		return
	}

	levelEntry, _ := components.Level.First(ecs.World)
	levelData := components.Level.Get(levelEntry)

	player := components.Player.Get(e)
	obj := components.Object.Get(e)
	space := components.Space.Get(spaceEntry)

	// Try to find best spawn point (farthest from enemies and death location)
	bestSpawn := findBestRespawnPoint(ecs, e, levelData.CurrentLevel.PlayerSpawns, space)

	if bestSpawn != nil {
		resetPlayerAtPosition(e, bestSpawn.X, bestSpawn.Y)
		ResetMessageState(ecs)
		return
	}

	// Fallback: use last safe position or original spawn
	spawnX, spawnY := player.LastSafeX, player.LastSafeY
	if spawnX == 0 && spawnY == 0 {
		spawnX, spawnY = player.OriginalSpawnX, player.OriginalSpawnY
	}
	if spawnX == 0 && spawnY == 0 {
		spawnX, spawnY = obj.X, obj.Y
	}

	if !isPositionSafe(space, spawnX, spawnY, obj.W, obj.H) {
		x, y, found := findNearestSafeGround(space, obj.X, obj.Y, obj.W, obj.H)
		if !found {
			RespawnPlayer(ecs, e)
			return
		}
		spawnX, spawnY = x, y
	}

	resetPlayerAtPosition(e, spawnX, spawnY)

	// Reset message state so messages can be shown again
	ResetMessageState(ecs)
}

func resetPlayerAtPosition(e *donburi.Entry, spawnX, spawnY float64) {
	obj := components.Object.Get(e)
	obj.X = spawnX
	obj.Y = spawnY

	physics := components.Physics.Get(e)
	physics.SpeedX = 0
	physics.SpeedY = 0
	physics.OnGround = nil
	physics.WallSliding = nil
	physics.IgnorePlatform = nil

	player := components.Player.Get(e)
	player.InvulnFrames = cfg.Player.RespawnInvulnFrames

	state := components.State.Get(e)
	state.CurrentState = cfg.Idle
	state.StateTimer = 0

	health := components.Health.Get(e)
	health.Current = health.Max

	// Re-add Flash component if it was removed during death sequence
	if !e.HasComponent(components.Flash) {
		e.AddComponent(components.Flash)
		components.Flash.SetValue(e, components.FlashData{
			Duration: 0,
			R:        1, G: 1, B: 1,
		})
	}
}

func isPositionSafe(space *resolv.Space, x, y, w, h float64) bool {
	tempObj := resolv.NewObject(x, y, w, h)
	space.Add(tempObj)
	defer space.Remove(tempObj)

	if tempObj.Check(0, 0, tags.ResolvDeadZone) != nil {
		return false
	}
	return tempObj.Check(0, 2, tags.ResolvSolid, "platform", tags.ResolvRamp) != nil
}

func findNearestSafeGround(space *resolv.Space, startX, startY, w, h float64) (x, y float64, found bool) {
	const searchStep = 32.0
	const maxSearchDist = 512.0

	// Reuse a single object for all checks to avoid allocations
	tempObj := resolv.NewObject(startX, startY, w, h)
	space.Add(tempObj)
	defer space.Remove(tempObj)

	checkSafe := func(checkX, checkY float64) bool {
		tempObj.X = checkX
		tempObj.Y = checkY
		if tempObj.Check(0, 0, tags.ResolvDeadZone) != nil {
			return false
		}
		return tempObj.Check(0, 2, tags.ResolvSolid, "platform", tags.ResolvRamp) != nil
	}

	// Search left then right
	for _, dir := range []float64{-1, 1} {
		for dist := searchStep; dist <= maxSearchDist; dist += searchStep {
			checkX := startX + dist*dir
			for checkY := startY - 64; checkY <= startY+128; checkY += 16 {
				if checkSafe(checkX, checkY) {
					return checkX, checkY, true
				}
			}
		}
	}

	return 0, 0, false
}

// findBestRespawnPoint finds the spawn point farthest from enemies and death location.
// Returns nil if no safe spawn point is found.
func findBestRespawnPoint(ecs *ecs.ECS, deadPlayer *donburi.Entry, spawns []assets.PlayerSpawn, space *resolv.Space) *assets.PlayerSpawn {
	if len(spawns) == 0 {
		return nil
	}

	deadObj := components.Object.Get(deadPlayer)
	deadX, deadY := deadObj.X, deadObj.Y
	deadPlayerData := components.Player.Get(deadPlayer)

	// Collect positions of living players (excluding the dead one)
	var livingPositions []struct{ x, y float64 }
	tags.Player.Each(ecs.World, func(entry *donburi.Entry) {
		if entry == deadPlayer {
			return
		}
		if entry.HasComponent(components.Death) {
			return // Skip other dead players
		}
		obj := components.Object.Get(entry)
		livingPositions = append(livingPositions, struct{ x, y float64 }{obj.X, obj.Y})
	})

	const minDistFromPlayers = 150.0
	var bestSpawn *assets.PlayerSpawn
	bestScore := -1.0

	for i := range spawns {
		spawn := &spawns[i]

		// Check if spawn is safe
		if !isPositionSafe(space, spawn.X, spawn.Y, deadObj.W, deadObj.H) {
			continue
		}

		// Calculate distance from death location
		deathDist := distance(spawn.X, spawn.Y, deadX, deadY)

		// Calculate minimum distance from living players
		minPlayerDist := 9999.0
		for _, pos := range livingPositions {
			d := distance(spawn.X, spawn.Y, pos.x, pos.y)
			if d < minPlayerDist {
				minPlayerDist = d
			}
		}

		// Skip if too close to living players
		if minPlayerDist < minDistFromPlayers && len(livingPositions) > 0 {
			continue
		}

		// Score: prefer far from death, far from players
		score := deathDist + minPlayerDist

		if score > bestScore {
			bestScore = score
			bestSpawn = spawn
		}
	}

	// If no safe spawn found, use original spawn
	if bestSpawn == nil && deadPlayerData.OriginalSpawnX != 0 {
		return &assets.PlayerSpawn{
			X: deadPlayerData.OriginalSpawnX,
			Y: deadPlayerData.OriginalSpawnY,
		}
	}

	return bestSpawn
}
