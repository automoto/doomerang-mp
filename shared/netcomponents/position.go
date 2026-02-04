package netcomponents

import "github.com/yohamta/donburi"

type NetPositionData struct {
	X, Y float64
}

var NetPosition = donburi.NewComponentType[NetPositionData]()

// LerpNetPosition interpolates between two positions
func LerpNetPosition(from, to NetPositionData, t float64) *NetPositionData {
	return &NetPositionData{
		X: from.X + (to.X-from.X)*t,
		Y: from.Y + (to.Y-from.Y)*t,
	}
}
