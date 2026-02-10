package core

import (
	"log"
	"math"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/gamemath"
	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/automoto/doomerang-mp/tags"
	"github.com/leap-fish/necs/esync"
	"github.com/leap-fish/necs/esync/srvsync"
	"github.com/yohamta/donburi"
)

// updateBoomerangs is called from updatePhysics() each server tick.
func (s *Server) updateBoomerangs() {
	stepsPerTick := 60 / s.loop.tickRate
	if stepsPerTick < 1 {
		stepsPerTick = 1
	}

	// Process charge state per player
	for entity, pp := range s.playerPhysics {
		if !s.world.Valid(entity) {
			continue
		}
		s.processBoomerangCharge(entity, pp)
	}

	// Sub-stepped boomerang physics
	for step := 0; step < stepsPerTick; step++ {
		for bEntity, bp := range s.boomerangPhysics {
			if bp.Destroy {
				continue
			}
			if !s.world.Valid(bEntity) {
				bp.Destroy = true
				continue
			}
			s.stepBoomerangPhysics(bEntity, bp)
			s.checkBoomerangCollisions(bEntity, bp)
		}
	}

	// Write final positions to net components
	for bEntity, bp := range s.boomerangPhysics {
		if bp.Destroy || !s.world.Valid(bEntity) {
			continue
		}
		entry := s.world.Entry(bEntity)
		nb := netcomponents.NetBoomerang.Get(entry)
		nb.X = bp.Object.X
		nb.Y = bp.Object.Y
		nb.VelX = bp.VelX
		nb.VelY = bp.VelY
		nb.State = bp.State
		nb.DistanceTraveled = bp.DistanceTraveled
		nb.ChargeRatio = bp.ChargeRatio
	}

	// Deferred removal
	s.destroyFlaggedBoomerangs()
}

// boomChargeVFXFrame is the charge frame at which the charge VFX is spawned on clients.
const boomChargeVFXFrame = 15

func (s *Server) processBoomerangCharge(entity donburi.Entity, pp *PlayerPhysics) {
	// Skip if player already has an active boomerang
	if _, active := s.playerBoomerangs[entity]; active {
		pp.BoomerangWasPressed = pp.BoomerangPressed
		return
	}

	entry := s.world.Entry(entity)

	// Edge detect: press start → begin charging
	if pp.BoomerangPressed && !pp.BoomerangWasPressed {
		pp.BoomerangCharging = true
		pp.BoomerangChargeTime = 0
	}

	// While held: increment charge
	if pp.BoomerangCharging && pp.BoomerangPressed {
		pp.BoomerangChargeTime++
		if pp.BoomerangChargeTime > cfg.Boomerang.MaxChargeTime {
			pp.BoomerangChargeTime = cfg.Boomerang.MaxChargeTime
		}

		// Set charging animation state
		if entry.HasComponent(netcomponents.NetPlayerState) {
			state := netcomponents.NetPlayerState.Get(entry)
			state.StateID = netconfig.StateChargingBoomerang
		}

		// Broadcast charge VFX event after holding for a bit (matches offline: frame 15)
		if pp.BoomerangChargeTime == boomChargeVFXFrame {
			var ownerNetID uint
			if nid := esync.GetNetworkId(entry); nid != nil {
				ownerNetID = uint(*nid)
			}
			s.broadcastEvent(messages.BoomerangChargeEvent{
				OwnerNetworkID: ownerNetID,
				X:              pp.Object.X + pp.Object.W/2,
				Y:              pp.Object.Y + pp.Object.H,
			})
		}
	}

	// Release: throw
	if pp.BoomerangCharging && !pp.BoomerangPressed {
		s.throwBoomerang(entity, pp)
		pp.BoomerangCharging = false
		pp.BoomerangChargeTime = 0
	}

	pp.BoomerangWasPressed = pp.BoomerangPressed
}

func (s *Server) throwBoomerang(playerEntity donburi.Entity, pp *PlayerPhysics) {
	if !s.world.Valid(playerEntity) {
		return
	}

	chargeRatio := float64(pp.BoomerangChargeTime) / float64(cfg.Boomerang.MaxChargeTime)

	// Calculate aim direction
	facingX := 1.0
	playerEntry := s.world.Entry(playerEntity)
	if playerEntry.HasComponent(netcomponents.NetPlayerState) {
		state := netcomponents.NetPlayerState.Get(playerEntry)
		if state.Direction < 0 {
			facingX = -1.0
		}
	}
	aimX, aimY := gamemath.CalculateAimDirection(facingX, pp.MoveUpPressed, pp.CrouchPressed, pp.Direction != 0)

	// Normalize aim vector
	mag := math.Sqrt(aimX*aimX + aimY*aimY)
	if mag > 0 {
		aimX /= mag
		aimY /= mag
	}

	// Spawn position: player center + offset in facing direction
	spawnX := pp.Object.X + pp.Object.W/2 + facingX*10 - 6 // center the 12x12 boomerang
	spawnY := pp.Object.Y + pp.Object.H/2 - 6

	// Speed scales with charge
	speed := gamemath.CalculateThrowSpeed(cfg.Boomerang.ThrowSpeed, chargeRatio)

	velX, velY := gamemath.CalculateThrowVelocity(aimX, aimY, speed, cfg.Boomerang.ThrowLift)

	// Create ECS entity
	bEntity := s.world.Create(netcomponents.NetBoomerang)
	bEntry := s.world.Entry(bEntity)

	ownerNetID := esync.GetNetworkId(playerEntry)
	var ownerNetIDVal uint
	if ownerNetID != nil {
		ownerNetIDVal = uint(*ownerNetID)
	}

	netcomponents.NetBoomerang.Set(bEntry, &netcomponents.NetBoomerangData{
		X:              spawnX,
		Y:              spawnY,
		VelX:           velX,
		VelY:           velY,
		OwnerNetworkID: ownerNetIDVal,
		State:          netconfig.BoomerangOutbound,
		ChargeRatio:    chargeRatio,
	})

	// Create server-side physics
	bp := newBoomerangPhysics(s.activeLevel, spawnX, spawnY, playerEntity, ownerNetIDVal)
	bp.VelX = velX
	bp.VelY = velY
	bp.State = netconfig.BoomerangOutbound
	bp.MaxRange = gamemath.CalculateMaxRange(cfg.Boomerang.BaseRange, cfg.Boomerang.MaxChargeRange, chargeRatio)
	bp.PierceDistance = cfg.Boomerang.PierceDistance
	bp.Damage = gamemath.CalculateDamage(cfg.Boomerang.BaseDamage, cfg.Boomerang.MaxChargeDamageBonus, chargeRatio)
	bp.ChargeRatio = chargeRatio

	s.boomerangPhysics[bEntity] = bp
	s.playerBoomerangs[playerEntity] = bEntity

	// Register for network sync
	if err := srvsync.NetworkSync(s.world, &bEntity, netcomponents.NetBoomerang); err != nil {
		log.Printf("Failed to sync boomerang: %v", err)
		return
	}

	// Set player state to throw animation (locked for a short duration)
	if playerEntry.HasComponent(netcomponents.NetPlayerState) {
		state := netcomponents.NetPlayerState.Get(playerEntry)
		state.StateID = netconfig.Throw
	}
	pp.LockedStateTimer = 6 // ~200ms at 30Hz ticks

	// Broadcast throw event
	s.broadcastEvent(messages.BoomerangThrowEvent{
		OwnerNetworkID: ownerNetIDVal,
		X:              spawnX,
		Y:              spawnY,
		DirectionX:     aimX,
		DirectionY:     aimY,
		ChargeLevel:    chargeRatio,
	})
}

func (s *Server) stepBoomerangPhysics(bEntity donburi.Entity, bp *BoomerangPhysics) {
	switch bp.State {
	case netconfig.BoomerangOutbound:
		// Apply gravity
		bp.VelY += cfg.Boomerang.Gravity

		// Track distance
		dx := math.Abs(bp.VelX)
		dy := math.Abs(bp.VelY)
		bp.DistanceTraveled += math.Sqrt(dx*dx + dy*dy)

		// Switch to inbound at max range
		if bp.DistanceTraveled >= bp.MaxRange {
			bp.State = netconfig.BoomerangInbound
		}

	case netconfig.BoomerangInbound:
		// Home toward owner
		if !s.world.Valid(bp.OwnerEntity) {
			bp.Destroy = true
			return
		}
		ownerPP, ok := s.playerPhysics[bp.OwnerEntity]
		if !ok {
			bp.Destroy = true
			return
		}

		targetX := ownerPP.Object.X + ownerPP.Object.W/2
		targetY := ownerPP.Object.Y + ownerPP.Object.H/2
		bp.VelX, bp.VelY = gamemath.CalculateHomingVelocity(
			bp.Object.X+6, bp.Object.Y+6,
			targetX, targetY,
			cfg.Boomerang.ReturnSpeed,
		)
	}

	// Move object
	bp.Object.X += bp.VelX
	bp.Object.Y += bp.VelY
	bp.Object.Update()
}

func (s *Server) checkBoomerangCollisions(bEntity donburi.Entity, bp *BoomerangPhysics) {
	if bp.Destroy {
		return
	}

	// Proximity-based catch runs first — always checked for inbound boomerangs
	// regardless of whether resolv detects an overlap (the 12x12 boomerang can
	// oscillate past the player at high speed without a frame-perfect overlap).
	if bp.State == netconfig.BoomerangInbound && s.world.Valid(bp.OwnerEntity) {
		if ownerPP, ok := s.playerPhysics[bp.OwnerEntity]; ok {
			cx := bp.Object.X + 6
			cy := bp.Object.Y + 6
			ox := ownerPP.Object.X + ownerPP.Object.W/2
			oy := ownerPP.Object.Y + ownerPP.Object.H/2
			dist := math.Sqrt((cx-ox)*(cx-ox) + (cy-oy)*(cy-oy))
			if dist < cfg.Boomerang.CatchRadius {
				s.catchBoomerang(bEntity, bp)
				return
			}
		}
	}

	check := bp.Object.Check(0, 0, tags.ResolvSolid, "player")
	if check == nil {
		return
	}

	// Wall collision → switch to inbound
	if bp.State == netconfig.BoomerangOutbound {
		if solids := check.ObjectsByTags(tags.ResolvSolid); len(solids) > 0 {
			bp.State = netconfig.BoomerangInbound
		}
	}

	// Player collision
	players := check.ObjectsByTags("player")
	for _, pObj := range players {
		// Find which player entity owns this resolv object
		var hitEntity donburi.Entity
		var hitFound bool
		for pEntity, pp := range s.playerPhysics {
			if pp.Object == pObj {
				hitEntity = pEntity
				hitFound = true
				break
			}
		}
		if !hitFound {
			continue
		}

		// Owner + inbound → catch (backup for proximity check above)
		if hitEntity == bp.OwnerEntity && bp.State == netconfig.BoomerangInbound {
			s.catchBoomerang(bEntity, bp)
			return
		}

		// Owner collision while outbound → skip
		if hitEntity == bp.OwnerEntity {
			continue
		}

		// Already hit this player?
		if _, already := bp.HitPlayers[hitEntity]; already {
			continue
		}

		// Hit enemy player
		s.hitPlayer(bEntity, bp, hitEntity)
	}
}

func (s *Server) hitPlayer(bEntity donburi.Entity, bp *BoomerangPhysics, targetEntity donburi.Entity) {
	bp.HitPlayers[targetEntity] = struct{}{}

	if !s.world.Valid(targetEntity) {
		return
	}
	targetEntry := s.world.Entry(targetEntity)

	// Apply damage
	if targetEntry.HasComponent(netcomponents.NetPlayerState) {
		state := netcomponents.NetPlayerState.Get(targetEntry)
		state.Health -= bp.Damage
		if state.Health < 0 {
			state.Health = 0
		}
		state.StateID = netconfig.Hit
	}
	// Lock hit state animation for a short duration
	if targetPP, ok := s.playerPhysics[targetEntity]; ok {
		targetPP.LockedStateTimer = 10 // ~333ms at 30Hz ticks
	}

	// Apply knockback
	knockX := bp.VelX
	if mag := math.Abs(knockX); mag > 0 {
		knockX = (knockX / mag) * cfg.Boomerang.HitKnockback
	}
	knockY := cfg.Boomerang.KnockbackUpwardForce

	if targetEntry.HasComponent(netcomponents.NetVelocity) {
		vel := netcomponents.NetVelocity.Get(targetEntry)
		vel.SpeedX += knockX
		vel.SpeedY += knockY
	}

	// Hit position for VFX
	hitX := bp.Object.X + 6
	hitY := bp.Object.Y + 6

	var targetNetID uint
	if nid := esync.GetNetworkId(targetEntry); nid != nil {
		targetNetID = uint(*nid)
	}

	s.broadcastEvent(messages.BoomerangHitEvent{
		AttackerNetworkID: bp.OwnerNetworkID,
		TargetNetworkID:   targetNetID,
		HitX:              hitX,
		HitY:              hitY,
		ChargeRatio:       bp.ChargeRatio,
		Damage:            bp.Damage,
		KnockbackX:        knockX,
		KnockbackY:        knockY,
	})

	// Pierce: switch to inbound after pierce distance
	bp.PierceDistance -= 12 // reduce per hit
	if bp.PierceDistance <= 0 {
		bp.State = netconfig.BoomerangInbound
	}
}

func (s *Server) catchBoomerang(bEntity donburi.Entity, bp *BoomerangPhysics) {
	bp.Destroy = true

	s.broadcastEvent(messages.BoomerangCatchEvent{
		OwnerNetworkID: bp.OwnerNetworkID,
	})
}

// destroyBoomerang immediately cleans up a boomerang entity.
func (s *Server) destroyBoomerang(bEntity donburi.Entity) {
	if bp, ok := s.boomerangPhysics[bEntity]; ok {
		removeBoomerangPhysics(s.activeLevel, bp)
		delete(s.boomerangPhysics, bEntity)
		delete(s.playerBoomerangs, bp.OwnerEntity)
	}
	if s.world.Valid(bEntity) {
		s.world.Remove(bEntity)
	}
}

func (s *Server) destroyFlaggedBoomerangs() {
	for bEntity, bp := range s.boomerangPhysics {
		if bp.Destroy {
			s.destroyBoomerang(bEntity)
		}
	}
}
