package systems

import (
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/network"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/systems/factory"
	"github.com/leap-fish/necs/esync"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

// NewNetBoomerangEventSystem returns an ECS system that drains boomerang events
// from the network client and triggers VFX/SFX each tick.
func NewNetBoomerangEventSystem(client *network.Client) func(*ecs.ECS) {
	// Track charge VFX per player (ownerNetworkID → VFX entry)
	chargeVFX := make(map[uint]*donburi.Entry)

	return func(e *ecs.ECS) {
		// Charge events: spawn charge VFX at player feet
		for _, evt := range client.DrainChargeEvents() {
			PlaySFX(e, cfg.SoundBoomerangCharge)
			vfx := factory.SpawnChargeVFX(e, evt.X, evt.Y)
			if vfx != nil {
				chargeVFX[evt.OwnerNetworkID] = vfx
			}
		}

		// Throw events: destroy charge VFX, play throw SFX, spawn muzzle flash
		for _, evt := range client.DrainThrowEvents() {
			if vfx, ok := chargeVFX[evt.OwnerNetworkID]; ok {
				factory.DestroyChargeVFX(e, vfx)
				delete(chargeVFX, evt.OwnerNetworkID)
			}
			PlaySFX(e, cfg.SoundBoomerangThrow)
			factory.SpawnGunshot(e, evt.X, evt.Y, evt.DirectionX)
		}

		// Catch events: play SFX, immediately remove boomerang entity
		for _, evt := range client.DrainCatchEvents() {
			// Destroy any lingering charge VFX
			if vfx, ok := chargeVFX[evt.OwnerNetworkID]; ok {
				factory.DestroyChargeVFX(e, vfx)
				delete(chargeVFX, evt.OwnerNetworkID)
			}

			PlaySFX(e, cfg.SoundBoomerangCatch)

			// Immediately remove the boomerang entity from the client world
			// so it disappears right away rather than waiting for the next snapshot.
			var toRemove []donburi.Entity
			esync.NetworkEntityQuery.Each(e.World, func(entry *donburi.Entry) {
				if !entry.HasComponent(netcomponents.NetBoomerang) {
					return
				}
				nb := netcomponents.NetBoomerang.Get(entry)
				if nb.OwnerNetworkID == evt.OwnerNetworkID {
					toRemove = append(toRemove, entry.Entity())
				}
			})
			for _, entity := range toRemove {
				e.World.Remove(entity)
			}
		}

		// Hit events: play impact SFX, spawn explosion VFX, screen shake
		for _, evt := range client.DrainHitEvents() {
			PlaySFX(e, cfg.SoundBoomerangImpact)
			explosionScale := 0.5 + evt.ChargeRatio*0.5
			factory.SpawnExplosion(e, evt.HitX, evt.HitY, explosionScale)
			TriggerScreenShake(e, cfg.ScreenShake.BoomerangIntensity, cfg.ScreenShake.BoomerangDuration)
		}

		// Clean up stale charge VFX entries (owner disconnected, etc.)
		for id, vfx := range chargeVFX {
			if vfx == nil || !vfx.Valid() {
				delete(chargeVFX, id)
			}
		}
	}
}
