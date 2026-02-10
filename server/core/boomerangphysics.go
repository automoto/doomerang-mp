package core

import (
	"github.com/automoto/doomerang-mp/tags"
	"github.com/solarlune/resolv"
	"github.com/yohamta/donburi"
)

// BoomerangPhysics holds server-side physics state for a boomerang entity.
type BoomerangPhysics struct {
	Object           *resolv.Object
	VelX, VelY       float64
	State            int // 0=Outbound, 1=Inbound
	DistanceTraveled float64
	MaxRange         float64
	PierceDistance   float64
	Damage           int
	ChargeRatio      float64
	OwnerEntity      donburi.Entity
	OwnerNetworkID   uint
	HitPlayers       map[donburi.Entity]struct{}
	Destroy          bool // Flagged for deferred removal
}

func newBoomerangPhysics(level *ServerLevel, x, y float64, ownerEntity donburi.Entity, ownerNetworkID uint) *BoomerangPhysics {
	obj := resolv.NewObject(x, y, 12, 12, tags.ResolvBoomerang)
	obj.SetShape(resolv.NewRectangle(0, 0, 12, 12))
	level.Space.Add(obj)

	return &BoomerangPhysics{
		Object:         obj,
		OwnerEntity:    ownerEntity,
		OwnerNetworkID: ownerNetworkID,
		HitPlayers:     make(map[donburi.Entity]struct{}),
	}
}

func removeBoomerangPhysics(level *ServerLevel, bp *BoomerangPhysics) {
	level.Space.Remove(bp.Object)
}
