package components

import "github.com/yohamta/donburi"

// NetInterpData stores interpolation state for smooth rendering of remote
// networked entities between server snapshots.
type NetInterpData struct {
	PrevX, PrevY     float64
	TargetX, TargetY float64
	T                float64
	Initialized      bool
	VelX, VelY       float64 // Velocity at snapshot (for extrapolation)
}

var NetInterp = donburi.NewComponentType[NetInterpData]()
