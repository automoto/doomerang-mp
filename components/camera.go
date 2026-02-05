package components

import (
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
)

type CameraData struct {
	Position   math.Vec2
	LookAheadX float64 // Current smoothed X offset for look-ahead
	Zoom       float64 // Current zoom level (1.0 = normal, 0.5 = 2x zoom out)
}

var Camera = donburi.NewComponentType[CameraData]()
