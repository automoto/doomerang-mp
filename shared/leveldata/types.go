// Package leveldata provides TMX level parsing shared between client and server.
// It has no dependencies on ebitengine, donburi, or resolv — pure data only.
package leveldata

// CollisionData holds all collision-relevant data parsed from a TMX level file.
type CollisionData struct {
	SolidRects  []SolidRect
	SpawnPoints []SpawnPoint
	MapWidth    int
	MapHeight   int
}

// SolidRect represents a solid collision tile.
type SolidRect struct {
	X, Y, W, H float64
	SlopeType   string // "", "45_up_right", "45_up_left"
}

// SpawnPoint represents a player spawn location.
type SpawnPoint struct {
	X, Y  float64
	Index int
}
