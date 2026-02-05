package systems

import (
	"github.com/automoto/doomerang-mp/components"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi/ecs"
)

func DrawLevel(ecs *ecs.ECS, screen *ebiten.Image) {
	// Get camera
	cameraEntry, ok := components.Camera.First(ecs.World)
	if !ok {
		return // No camera yet
	}
	camera := components.Camera.Get(cameraEntry)
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()

	// Safety check for zero zoom
	zoom := camera.Zoom
	if zoom == 0 {
		zoom = 1.0
	}

	opts := &ebiten.DrawImageOptions{}
	// Apply camera transform with zoom:
	// 1. Translate to camera-relative position
	// 2. Scale by zoom
	// 3. Center on screen
	opts.GeoM.Translate(-camera.Position.X, -camera.Position.Y)
	opts.GeoM.Scale(zoom, zoom)
	opts.GeoM.Translate(float64(width)/2, float64(height)/2)

	// Draw the level background
	levelEntry, ok := components.Level.First(ecs.World)
	if !ok {
		return
	}

	levelData := components.Level.Get(levelEntry)
	if levelData.CurrentLevel == nil {
		return
	}

	// Draw the background from the loaded level
	if levelData.CurrentLevel.Background != nil {
		screen.DrawImage(levelData.CurrentLevel.Background, opts)
	}

	// Ground objects are now drawn in the debug system when F1 debug mode is enabled
}
