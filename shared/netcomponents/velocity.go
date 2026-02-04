package netcomponents

import "github.com/yohamta/donburi"

type NetVelocityData struct {
	SpeedX, SpeedY float64
}

var NetVelocity = donburi.NewComponentType[NetVelocityData]()

// LerpNetVelocity interpolates between two velocities
func LerpNetVelocity(from, to NetVelocityData, t float64) *NetVelocityData {
	return &NetVelocityData{
		SpeedX: from.SpeedX + (to.SpeedX-from.SpeedX)*t,
		SpeedY: from.SpeedY + (to.SpeedY-from.SpeedY)*t,
	}
}
