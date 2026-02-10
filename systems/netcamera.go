package systems

import (
	"math"

	"github.com/automoto/doomerang-mp/components"
	"github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/leap-fish/necs/esync"
	"github.com/yohamta/donburi/ecs"
)

// NewNetCameraSystem returns an update system that follows the local player
// in networked mode. It reads NetPosition instead of components.Object.
func NewNetCameraSystem(localNetID func() esync.NetworkId) func(*ecs.ECS) {
	return func(e *ecs.ECS) {
		cameraEntry, ok := components.Camera.First(e.World)
		if !ok {
			return
		}
		camera := components.Camera.Get(cameraEntry)

		levelEntry, ok := components.Level.First(e.World)
		if !ok {
			return
		}
		levelData := components.Level.Get(levelEntry)
		if levelData.CurrentLevel == nil {
			return
		}

		// Find local player entity
		entity := esync.FindByNetworkId(e.World, localNetID())
		if !e.World.Valid(entity) {
			return
		}
		entry := e.World.Entry(entity)
		if !entry.HasComponent(netcomponents.NetPosition) {
			return
		}

		pos := netcomponents.NetPosition.Get(entry)
		targetX := pos.X
		targetY := pos.Y

		// Clamp to level bounds
		screenW := float64(config.C.Width)
		screenH := float64(config.C.Height)

		zoom := camera.Zoom
		if zoom == 0 {
			zoom = 1.0
		}

		visibleW := screenW / zoom
		visibleH := screenH / zoom

		minCameraX := visibleW / 2
		maxCameraX := float64(levelData.CurrentLevel.Width) - visibleW/2
		minCameraY := visibleH / 2
		maxCameraY := float64(levelData.CurrentLevel.Height) - visibleH/2

		if minCameraX > maxCameraX {
			minCameraX = float64(levelData.CurrentLevel.Width) / 2
			maxCameraX = minCameraX
		}
		if minCameraY > maxCameraY {
			minCameraY = float64(levelData.CurrentLevel.Height) / 2
			maxCameraY = minCameraY
		}

		targetX = math.Max(minCameraX, math.Min(maxCameraX, targetX))
		targetY = math.Max(minCameraY, math.Min(maxCameraY, targetY))

		// Smooth follow
		camera.Position.X += (targetX - camera.Position.X) * config.Camera.FollowSmoothing
		camera.Position.Y += (targetY - camera.Position.Y) * config.Camera.FollowSmoothing
	}
}
