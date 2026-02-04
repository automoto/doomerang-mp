package netcomponents

import "github.com/yohamta/donburi"

type NetEnemyData struct {
	X, Y     float64
	TypeName string // "Guard", "HeavyGuard", etc.
	State    int
	Health   int
}

var NetEnemy = donburi.NewComponentType[NetEnemyData]()

// LerpNetEnemy interpolates between two enemy states
func LerpNetEnemy(from, to NetEnemyData, t float64) *NetEnemyData {
	return &NetEnemyData{
		X:        from.X + (to.X-from.X)*t,
		Y:        from.Y + (to.Y-from.Y)*t,
		TypeName: to.TypeName,
		State:    to.State,
		Health:   to.Health,
	}
}
