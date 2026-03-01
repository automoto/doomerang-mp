package systems

import (
	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/network"
	"github.com/automoto/doomerang-mp/systems/factory"
	"github.com/leap-fish/necs/esync"
	"github.com/yohamta/donburi/ecs"
)

// NewNetCombatEventSystem returns an ECS system that drains melee combat,
// death, and respawn events from the network client and triggers VFX/SFX.
func NewNetCombatEventSystem(client *network.Client) func(*ecs.ECS) {
	return func(e *ecs.ECS) {
		// Attack initiation events: play punch/kick SFX
		for _, evt := range client.DrainMeleeAttackEvents() {
			if evt.IsPunch {
				PlaySFX(e, cfg.SoundPunch)
			} else {
				PlaySFX(e, cfg.SoundKick)
			}
		}

		// Melee hit events
		for _, evt := range client.DrainMeleeHitEvents() {
			PlaySFX(e, cfg.SoundHit)
			factory.SpawnHitExplosion(e, evt.HitX, evt.HitY, 0.6)
			TriggerScreenShake(e, cfg.ScreenShake.MeleeIntensity, cfg.ScreenShake.MeleeDuration)

			// Flash the target player
			targetEntity := esync.FindByNetworkId(e.World, esync.NetworkId(evt.TargetNetworkID))
			if e.World.Valid(targetEntity) {
				targetEntry := e.World.Entry(targetEntity)
				if targetEntry.HasComponent(components.Flash) {
					TriggerDamageFlash(targetEntry)
				}
			}
		}

		// Death events
		for range client.DrainDeathEvents() {
			PlaySFX(e, cfg.SoundDeath)
			TriggerScreenShake(e, cfg.ScreenShake.PlayerDamageIntensity, cfg.ScreenShake.PlayerDamageDuration)
		}

		// Respawn events
		for _, evt := range client.DrainRespawnEvents() {
			factory.SpawnExplosion(e, evt.X, evt.Y, 0.8)
		}
	}
}
