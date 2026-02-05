package netcomponents

import "github.com/yohamta/donburi"

type NetBoomerangData struct {
	X, Y             float64
	OwnerNetworkID   uint // NetworkId of owning player
	State            int  // Flying, Returning, etc.
	DistanceTraveled float64
}

var NetBoomerang = donburi.NewComponentType[NetBoomerangData]()

// LerpNetBoomerang interpolates between two boomerang states
func LerpNetBoomerang(from, to NetBoomerangData, t float64) *NetBoomerangData {
	return &NetBoomerangData{
		X:                from.X + (to.X-from.X)*t,
		Y:                from.Y + (to.Y-from.Y)*t,
		OwnerNetworkID:   to.OwnerNetworkID,
		State:            to.State,
		DistanceTraveled: to.DistanceTraveled,
	}
}
