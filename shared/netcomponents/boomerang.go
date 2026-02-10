package netcomponents

import "github.com/yohamta/donburi"

type NetBoomerangData struct {
	X, Y             float64
	VelX, VelY       float64 // Client extrapolation between snapshots
	OwnerNetworkID   uint    // NetworkId of owning player
	State            int     // 0=Outbound, 1=Inbound
	DistanceTraveled float64
	ChargeRatio      float64 // 0.0-1.0, for client VFX scaling
}

var NetBoomerang = donburi.NewComponentType[NetBoomerangData]()

// LerpNetBoomerang interpolates between two boomerang states
func LerpNetBoomerang(from, to NetBoomerangData, t float64) *NetBoomerangData {
	return &NetBoomerangData{
		X:                from.X + (to.X-from.X)*t,
		Y:                from.Y + (to.Y-from.Y)*t,
		VelX:             to.VelX,
		VelY:             to.VelY,
		OwnerNetworkID:   to.OwnerNetworkID,
		State:            to.State,
		DistanceTraveled: to.DistanceTraveled,
		ChargeRatio:      to.ChargeRatio,
	}
}
