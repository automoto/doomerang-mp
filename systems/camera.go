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

	// Gather all player positions and track bounding box
	var sumX, sumY float64
	var playerCount int
	var minPlayerX, maxPlayerX, minPlayerY, maxPlayerY float64
	var singlePlayerPhysics *components.PhysicsData
	var singlePlayerData *components.PlayerData
	first := true

	tags.Player.Each(e.World, func(playerEntry *donburi.Entry) {
		playerObject := components.Object.Get(playerEntry)
		px := playerObject.X + playerObject.W/2
		py := playerObject.Y + playerObject.H/2

		sumX += px
		sumY += py
		playerCount++

		// Track bounding box of all players
		if first {
			minPlayerX, maxPlayerX = px, px
			minPlayerY, maxPlayerY = py, py
			first = false
			singlePlayerPhysics = components.Physics.Get(playerEntry)
			singlePlayerData = components.Player.Get(playerEntry)
		} else {
			if px < minPlayerX {
				minPlayerX = px
			}
			if px > maxPlayerX {
				maxPlayerX = px
			}
			if py < minPlayerY {
				minPlayerY = py
			}
			if py > maxPlayerY {
				maxPlayerY = py
			}
		}
	})

	if playerCount == 0 {
		return // no players (could be dead), skip camera update
	}

	screenWidth := float64(config.C.Width)
	screenHeight := float64(config.C.Height)

	// Calculate dynamic zoom to fit ALL players (2, 3, or 4)
	targetZoom := config.Camera.MaxZoom
	if playerCount > 1 {
		// Bounding box already includes ALL players from the loop above
		playerSpreadX := maxPlayerX - minPlayerX + config.Camera.ZoomMargin*2
		playerSpreadY := maxPlayerY - minPlayerY + config.Camera.ZoomMargin*2

		// Calculate zoom needed to fit ALL players (smaller value = more zoomed out)
		zoomX := screenWidth / playerSpreadX
		zoomY := screenHeight / playerSpreadY
		targetZoom = math.Min(zoomX, zoomY) // Use the more restrictive axis

		// Clamp zoom to configured limits
		targetZoom = math.Max(config.Camera.MinZoom, math.Min(config.Camera.MaxZoom, targetZoom))
	}

	// Safety check for zero zoom
	if camera.Zoom == 0 {
		camera.Zoom = 1.0
	}

	// Smoothly interpolate zoom
	camera.Zoom += (targetZoom - camera.Zoom) * config.Camera.ZoomSmoothing

	// Calculate target position
	var targetX, targetY float64

	if playerCount == 1 && singlePlayerPhysics != nil && singlePlayerData != nil {
		// Single player: use existing look-ahead logic
		centerX := sumX / float64(playerCount)
		centerY := sumY / float64(playerCount)

		if math.Abs(singlePlayerPhysics.SpeedX) > config.Camera.LookAheadSpeedThreshold {
			targetLookAhead := singlePlayerData.Direction.X * config.Camera.LookAheadDistanceX * config.Camera.LookAheadMovingScale
			camera.LookAheadX += (targetLookAhead - camera.LookAheadX) * config.Camera.LookAheadSmoothing
		}
		targetX = centerX + camera.LookAheadX
		targetY = centerY
	} else {
		// Multiplayer: center on the midpoint of all players
		camera.LookAheadX *= 0.9 // Decay look-ahead

		// When zoomed out (or zooming), center on the midpoint of all players
		// This ensures all players stay visible without fighting with dead zone logic
		centerX := (minPlayerX + maxPlayerX) / 2
		centerY := (minPlayerY + maxPlayerY) / 2

		targetX = centerX
		targetY = centerY
	}

	// Calculate camera bounds based on level dimensions AND zoom
	levelWidth := float64(levelData.CurrentLevel.Width)
	levelHeight := float64(levelData.CurrentLevel.Height)

	// Visible area is larger when zoomed out
	visibleW := screenWidth / camera.Zoom
	visibleH := screenHeight / camera.Zoom

	minCameraX := visibleW / 2
	maxCameraX := levelWidth - visibleW/2
	minCameraY := visibleH / 2
	maxCameraY := levelHeight - visibleH/2

	// Handle case where level is smaller than visible area
	if minCameraX > maxCameraX {
		minCameraX = levelWidth / 2
		maxCameraX = levelWidth / 2
	}
	if minCameraY > maxCameraY {
		minCameraY = levelHeight / 2
		maxCameraY = levelHeight / 2
	}

	// Constrain target position to camera bounds
	targetX = math.Max(minCameraX, math.Min(maxCameraX, targetX))
	targetY = math.Max(minCameraY, math.Min(maxCameraY, targetY))

	// Apply smoothing
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
