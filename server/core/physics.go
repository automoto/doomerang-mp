package core

import (
	"math"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/gamemath"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/automoto/doomerang-mp/tags"
	"github.com/solarlune/resolv"
)

// updatePhysics runs sub-stepped physics for all players. Called once per
// server tick. Sub-stepping ensures the same physics constants that were tuned
// for 60 Hz work correctly at the server's lower tick rate.
func (s *Server) updatePhysics() {
	stepsPerTick := 60 / s.loop.tickRate // e.g. 2 at 30 Hz
	if stepsPerTick < 1 {
		stepsPerTick = 1
	}

	for step := 0; step < stepsPerTick; step++ {
		for entity, pp := range s.playerPhysics {
			if !s.world.Valid(entity) {
				continue
			}
			entry := s.world.Entry(entity)
			vel := netcomponents.NetVelocity.Get(entry)
			s.stepPlayerPhysics(pp, vel)
		}
	}

	// Update boomerangs (charge, physics, collision, writeback)
	s.updateBoomerangs()

	// After all sub-steps, write final positions and derive state.
	for entity, pp := range s.playerPhysics {
		if !s.world.Valid(entity) {
			continue
		}
		entry := s.world.Entry(entity)
		pos := netcomponents.NetPosition.Get(entry)
		vel := netcomponents.NetVelocity.Get(entry)
		state := netcomponents.NetPlayerState.Get(entry)

		pos.X = pp.Object.X
		pos.Y = pp.Object.Y
		// Preserve locked states (charging, throw, hit) set by other systems
		if !isLockedServerState(pp, state.StateID) {
			state.StateID = deriveState(pp, vel)
		}
		state.LastSequence = pp.LastInputSeq
	}
}

// stepPlayerPhysics performs a single 60 Hz physics sub-step for one player.
func (s *Server) stepPlayerPhysics(pp *PlayerPhysics, vel *netcomponents.NetVelocityData) {
	// Skip acceleration during charging — friction only, matching offline
	if pp.Direction != 0 && !pp.BoomerangCharging {
		vel.SpeedX += float64(pp.Direction) * cfg.Player.Acceleration
	}

	// --- Jump (edge-triggered) ---
	if pp.JumpPressed && !pp.JumpWasPressed && pp.OnGround {
		vel.SpeedY = -cfg.Player.JumpSpeed
		pp.OnGround = false
	}
	pp.JumpWasPressed = pp.JumpPressed

	// --- Friction (ground only) ---
	if pp.OnGround {
		vel.SpeedX = gamemath.ApplyFriction(vel.SpeedX, cfg.Player.Friction)
	}

	// --- Clamp horizontal speed ---
	vel.SpeedX = gamemath.ClampSpeed(vel.SpeedX, cfg.Player.MaxSpeed)

	// --- Gravity ---
	vel.SpeedY += cfg.Physics.Gravity
	if vel.SpeedY > cfg.Physics.MaxFallSpeed {
		vel.SpeedY = cfg.Physics.MaxFallSpeed
	}

	// --- Resolve horizontal collision ---
	dx := vel.SpeedX
	if dx != 0 && !tryHorizontalRamp(pp, vel, dx) {
		// Solid wall check (ramps don't block horizontally)
		if check := pp.Object.Check(dx, 0, tags.ResolvSolid); check != nil {
			if solids := check.ObjectsByTags(tags.ResolvSolid); len(solids) > 0 {
				contact := check.ContactWithObject(solids[0])
				dx = contact.X()
				vel.SpeedX = 0
			}
		}
		pp.Object.X += dx
	}

	// --- Resolve vertical collision ---
	dy := vel.SpeedY
	// Hard clamp per client (systems/collision.go)
	dy = math.Max(math.Min(dy, cfg.Physics.MaxVertSpeed), -cfg.Physics.MaxVertSpeed)

	checkDist := dy
	if dy >= 0 {
		checkDist++
	}

	if check := pp.Object.Check(0, checkDist, tags.ResolvSolid, tags.ResolvRamp); check != nil {
		// Check ramps first (priority, matching client)
		if dy >= 0 {
			if ramps := check.ObjectsByTags(tags.ResolvRamp); len(ramps) > 0 {
				surfaceY := gamemath.GetSlopeSurfaceY(pp.Object, ramps[0], tags.Slope45UpRight, tags.Slope45UpLeft)
				playerBottom := pp.Object.Y + pp.Object.H
				if playerBottom+dy >= surfaceY {
					pp.Object.Y = gamemath.SnapToSlopeY(pp.Object.H, surfaceY, cfg.Physics.SlopeSurfaceOffset)
					vel.SpeedY = 0
					pp.OnGround = true
					return
				}
			}
		}

		if solids := check.ObjectsByTags(tags.ResolvSolid); len(solids) > 0 {
			if dy >= 0 {
				// Landing
				contact := check.ContactWithObject(solids[0])
				pp.Object.Y += contact.Y()
				vel.SpeedY = 0
				pp.OnGround = true
				return
			}
			// Hitting ceiling
			contact := check.ContactWithObject(solids[0])
			pp.Object.Y += contact.Y()
			vel.SpeedY = 0
			return
		}
	}

	// No collision — freefall
	pp.OnGround = false
	pp.Object.Y += dy
}

// tryHorizontalRamp checks for ramp collision when moving horizontally.
// Returns true if the player was snapped to a ramp (horizontal movement handled).
func tryHorizontalRamp(pp *PlayerPhysics, vel *netcomponents.NetVelocityData, dx float64) bool {
	// Check for ramp collision in front (walking uphill)
	if rampCheck := pp.Object.Check(dx, 0, tags.ResolvRamp); rampCheck != nil {
		if ramps := rampCheck.ObjectsByTags(tags.ResolvRamp); len(ramps) > 0 {
			pp.Object.X += dx
			snapToSlope(pp, vel, ramps[0])
			return true
		}
	}
	// Check for ramp below (walking downhill)
	if rampCheck := pp.Object.Check(dx, 1, tags.ResolvRamp); rampCheck != nil {
		if ramps := rampCheck.ObjectsByTags(tags.ResolvRamp); len(ramps) > 0 {
			pp.Object.X += dx
			snapToSlope(pp, vel, ramps[0])
			return true
		}
	}
	return false
}

// isLockedServerState returns true for states that should not be overwritten by deriveState.
func isLockedServerState(pp *PlayerPhysics, state netconfig.StateID) bool {
	if pp.BoomerangCharging {
		return true // Currently charging — preserve StateChargingBoomerang
	}
	if pp.LockedStateTimer > 0 {
		pp.LockedStateTimer--
		return true // Throw/Hit animation still playing
	}
	return false
}

// deriveState maps physics state to a NetPlayerState animation state.
func deriveState(pp *PlayerPhysics, vel *netcomponents.NetVelocityData) netconfig.StateID {
	if !pp.OnGround {
		return netconfig.Jump
	}
	if math.Abs(vel.SpeedX) >= 0.1 {
		return netconfig.Running
	}
	return netconfig.Idle
}

// snapToSlope adjusts the player Y position to stay on a slope surface.
func snapToSlope(pp *PlayerPhysics, vel *netcomponents.NetVelocityData, ramp *resolv.Object) {
	surfaceY := gamemath.GetSlopeSurfaceY(pp.Object, ramp, tags.Slope45UpRight, tags.Slope45UpLeft)
	pp.Object.Y = gamemath.SnapToSlopeY(pp.Object.H, surfaceY, cfg.Physics.SlopeSurfaceOffset)
	pp.OnGround = true
	vel.SpeedY = 0
}
