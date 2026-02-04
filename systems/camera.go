package systems

import (
	"math"

	"github.com/automoto/doomerang-mp/components"
	"github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/tags"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

func UpdateCamera(e *ecs.ECS) {
	cameraEntry, _ := components.Camera.First(e.World)
	camera := components.Camera.Get(cameraEntry)

	// Process screen shake
	updateScreenShake(cameraEntry, camera)

	// Get level dimensions for camera bounds
	levelEntry, ok := components.Level.First(e.World)
	if !ok {
		return
	}
	levelData := components.Level.Get(levelEntry)
	if levelData.CurrentLevel == nil {
		return
	}

	// Gather all player positions
	var sumX, sumY float64
	var playerCount int
	var singlePlayerPhysics *components.PhysicsData
	var singlePlayerData *components.PlayerData

	tags.Player.Each(e.World, func(playerEntry *donburi.Entry) {
		playerObject := components.Object.Get(playerEntry)
		sumX += playerObject.X + playerObject.W/2
		sumY += playerObject.Y + playerObject.H/2
		playerCount++
		// Keep reference to first player for look-ahead (only used in single-player mode)
		if playerCount == 1 {
			singlePlayerPhysics = components.Physics.Get(playerEntry)
			singlePlayerData = components.Player.Get(playerEntry)
		}
	})

	if playerCount == 0 {
		return // no players (could be dead), skip camera update
	}

	// Calculate center point of all players
	centerX := sumX / float64(playerCount)
	centerY := sumY / float64(playerCount)

	// Only apply look-ahead in single-player mode
	if playerCount == 1 && singlePlayerPhysics != nil && singlePlayerData != nil {
		if math.Abs(singlePlayerPhysics.SpeedX) > config.Camera.LookAheadSpeedThreshold {
			targetLookAhead := singlePlayerData.Direction.X * config.Camera.LookAheadDistanceX * config.Camera.LookAheadMovingScale
			camera.LookAheadX += (targetLookAhead - camera.LookAheadX) * config.Camera.LookAheadSmoothing
		}
		centerX += camera.LookAheadX
	} else {
		// Decay look-ahead in multiplayer mode
		camera.LookAheadX *= 0.9
	}

	// Calculate camera bounds based on screen and level dimensions
	screenWidth := float64(config.C.Width)
	screenHeight := float64(config.C.Height)
	levelWidth := float64(levelData.CurrentLevel.Width)
	levelHeight := float64(levelData.CurrentLevel.Height)

	// Camera bounds: ensure the level always fills the screen
	minCameraX := screenWidth / 2
	maxCameraX := levelWidth - screenWidth/2
	minCameraY := screenHeight / 2
	maxCameraY := levelHeight - screenHeight/2

	// Constrain target position to camera bounds
	targetX := math.Max(minCameraX, math.Min(maxCameraX, centerX))
	targetY := math.Max(minCameraY, math.Min(maxCameraY, centerY))

	// Center the camera on the constrained target position, with some smoothing.
	camera.Position.X += (targetX - camera.Position.X) * config.Camera.FollowSmoothing
	camera.Position.Y += (targetY - camera.Position.Y) * config.Camera.FollowSmoothing
}

// updateScreenShake applies screen shake offset to camera and decrements duration
func updateScreenShake(cameraEntry *donburi.Entry, camera *components.CameraData) {
	if !cameraEntry.HasComponent(components.ScreenShake) {
		return
	}

	shake := components.ScreenShake.Get(cameraEntry)
	shake.Elapsed++

	// Calculate decaying intensity
	progress := float64(shake.Duration-shake.Elapsed) / float64(shake.Duration)
	if progress < 0 {
		progress = 0
	}
	currentIntensity := shake.Intensity * progress

	// Apply oscillating offset using sine/cosine for smooth shake
	offsetX := math.Sin(float64(shake.Elapsed)*1.1) * currentIntensity
	offsetY := math.Cos(float64(shake.Elapsed)*1.3) * currentIntensity

	camera.Position.X += offsetX
	camera.Position.Y += offsetY

	// Remove component when shake is complete
	if shake.Elapsed >= shake.Duration {
		cameraEntry.RemoveComponent(components.ScreenShake)
	}
}

// TriggerScreenShake starts a screen shake effect
func TriggerScreenShake(ecs *ecs.ECS, intensity float64, duration int) {
	cameraEntry, ok := components.Camera.First(ecs.World)
	if !ok {
		return
	}

	// Add or update screen shake component
	if cameraEntry.HasComponent(components.ScreenShake) {
		shake := components.ScreenShake.Get(cameraEntry)
		// Only override if new shake is stronger
		if intensity > shake.Intensity {
			shake.Intensity = intensity
			shake.Duration = duration
			shake.Elapsed = 0
		}
	} else {
		cameraEntry.AddComponent(components.ScreenShake)
		components.ScreenShake.Set(cameraEntry, &components.ScreenShakeData{
			Intensity: intensity,
			Duration:  duration,
			Elapsed:   0,
		})
	}
}
