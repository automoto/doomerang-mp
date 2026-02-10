package systems

import (
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/automoto/doomerang-mp/systems/factory"
	"github.com/leap-fish/necs/esync"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

func isGroundState(s netconfig.StateID) bool {
	return s == netconfig.Idle || s == netconfig.Running
}

// NewNetPlayerEffectsSystem detects jump/land transitions and triggers
// SFX, dust VFX, and squash/stretch for networked players.
func NewNetPlayerEffectsSystem(prediction *NetPrediction, localNetID func() esync.NetworkId) func(*ecs.ECS) {
	prevStates := make(map[esync.NetworkId]netconfig.StateID)
	seen := make(map[esync.NetworkId]bool)

	collisionW := float64(cfg.Player.CollisionWidth)
	collisionH := float64(cfg.Player.CollisionHeight)

	return func(e *ecs.ECS) {
		myID := localNetID()
		clear(seen)

		esync.NetworkEntityQuery.Each(e.World, func(entry *donburi.Entry) {
			if !entry.HasComponent(netcomponents.NetPlayerState) || !entry.HasComponent(netcomponents.NetPosition) {
				return
			}
			nid := esync.GetNetworkId(entry)
			if nid == nil {
				return
			}
			id := *nid
			seen[id] = true

			pos := netcomponents.NetPosition.Get(entry)
			feetX := pos.X + collisionW/2
			feetY := pos.Y + collisionH

			if id == myID {
				if prediction == nil || !prediction.Initialized {
					return
				}
				// VelY < 0 filters false positives from reconciliation at jump apex
				if prediction.WasOnGround && !prediction.OnGround && prediction.VelY < 0 {
					triggerJumpEffects(e, entry, feetX, feetY)
				} else if !prediction.WasOnGround && prediction.OnGround {
					triggerLandEffects(e, entry, feetX, feetY)
				}
				return
			}

			state := netcomponents.NetPlayerState.Get(entry)
			cur := state.StateID
			prev, hasPrev := prevStates[id]
			prevStates[id] = cur
			if !hasPrev {
				return
			}

			if isGroundState(prev) && cur == netconfig.Jump {
				triggerJumpEffects(e, entry, feetX, feetY)
			} else if prev == netconfig.Jump && isGroundState(cur) {
				triggerLandEffects(e, entry, feetX, feetY)
			}
		})

		for id := range prevStates {
			if !seen[id] {
				delete(prevStates, id)
			}
		}
	}
}

func triggerJumpEffects(e *ecs.ECS, entry *donburi.Entry, feetX, feetY float64) {
	PlaySFX(e, cfg.SoundJump)
	factory.SpawnJumpDust(e, feetX, feetY)
	TriggerSquashStretch(entry, cfg.SquashStretch.JumpScaleX, cfg.SquashStretch.JumpScaleY)
}

func triggerLandEffects(e *ecs.ECS, entry *donburi.Entry, feetX, feetY float64) {
	PlaySFX(e, cfg.SoundLand)
	factory.SpawnLandDust(e, feetX, feetY)
	TriggerSquashStretch(entry, cfg.SquashStretch.LandScaleX, cfg.SquashStretch.LandScaleY)
}
