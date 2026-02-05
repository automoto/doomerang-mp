package components

import "github.com/yohamta/donburi"

type DamageEventData struct {
	Amount        int
	KnockbackX    float64
	KnockbackY    float64
	AttackerIndex int // PlayerIndex of attacker for KO credit (-1 = environment/self)
}

var DamageEvent = donburi.NewComponentType[DamageEventData]()
