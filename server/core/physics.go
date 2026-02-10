package core

import (
	"math"

	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/solarlune/resolv"
)

// Physics constants — matching config/config.go values used by the client.
const (
	gravity            = 0.75
	jumpSpeed          = 15.0
	maxSpeed           = 6.0
	acceleration       = 0.75
	friction           = 0.5
	maxFallSpeed       = 10.0
	maxVertSpeed       = 16.0 // hard clamp from systems/collision.go:190
	slopeSurfaceOffset = 0.1  // matches systems/collision.go:18
)

// Collision tag strings — must match tags/tags.go values in the client module.
const (
	tagSolid      = "solid"
	tagRamp       = "ramp"
	tagSlope45UpR = "45_up_right"
	tagSlope45UpL = "45_up_left"
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
		state.StateID = deriveState(pp, vel)
		state.LastSequence = pp.LastInputSeq
	}
}

// stepPlayerPhysics performs a single 60 Hz physics sub-step for one player.
func (s *Server) stepPlayerPhysics(pp *PlayerPhysics, vel *netcomponents.NetVelocityData) {
	// --- Horizontal input ---
	if pp.Direction != 0 {
		vel.SpeedX += float64(pp.Direction) * acceleration
	}

	// --- Jump (edge-triggered) ---
	if pp.JumpPressed && !pp.JumpWasPressed && pp.OnGround {
		vel.SpeedY = -jumpSpeed
		pp.OnGround = false
	}
	pp.JumpWasPressed = pp.JumpPressed

	// --- Friction (ground only) ---
	if pp.OnGround {
		if vel.SpeedX > friction {
			vel.SpeedX -= friction
		} else if vel.SpeedX < -friction {
			vel.SpeedX += friction
		} else {
			vel.SpeedX = 0
		}
	}

	// --- Clamp horizontal speed ---
	if vel.SpeedX > maxSpeed {
		vel.SpeedX = maxSpeed
	} else if vel.SpeedX < -maxSpeed {
		vel.SpeedX = -maxSpeed
	}

	// --- Gravity ---
	vel.SpeedY += gravity
	if vel.SpeedY > maxFallSpeed {
		vel.SpeedY = maxFallSpeed
	}

	// --- Resolve horizontal collision ---
	dx := vel.SpeedX
	if dx != 0 && !tryHorizontalRamp(pp, vel, dx) {
		// Solid wall check (ramps don't block horizontally)
		if check := pp.Object.Check(dx, 0, tagSolid); check != nil {
			if solids := check.ObjectsByTags(tagSolid); len(solids) > 0 {
				contact := check.ContactWithObject(solids[0])
				dx = contact.X()
				vel.SpeedX = 0
			}
		}
		pp.Object.X += dx
	}

	// --- Resolve vertical collision ---
	dy := vel.SpeedY
	// Hard clamp per client (systems/collision.go:189-191)
	dy = math.Max(math.Min(dy, maxVertSpeed), -maxVertSpeed)

	checkDist := dy
	if dy >= 0 {
		checkDist++
	}

	if check := pp.Object.Check(0, checkDist, tagSolid, tagRamp); check != nil {
		// Check ramps first (priority, matching client)
		if dy >= 0 {
			if ramps := check.ObjectsByTags(tagRamp); len(ramps) > 0 {
				surfaceY := serverGetSlopeSurfaceY(pp.Object, ramps[0])
				playerBottom := pp.Object.Y + pp.Object.H
				if playerBottom+dy >= surfaceY {
					pp.Object.Y = surfaceY - pp.Object.H + slopeSurfaceOffset
					vel.SpeedY = 0
					pp.OnGround = true
					return
				}
			}
		}

		if solids := check.ObjectsByTags(tagSolid); len(solids) > 0 {
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
	if rampCheck := pp.Object.Check(dx, 0, tagRamp); rampCheck != nil {
		if ramps := rampCheck.ObjectsByTags(tagRamp); len(ramps) > 0 {
			pp.Object.X += dx
			serverSnapToSlope(pp, vel, ramps[0])
			return true
		}
	}
	// Check for ramp below (walking downhill)
	if rampCheck := pp.Object.Check(dx, 1, tagRamp); rampCheck != nil {
		if ramps := rampCheck.ObjectsByTags(tagRamp); len(ramps) > 0 {
			pp.Object.X += dx
			serverSnapToSlope(pp, vel, ramps[0])
			return true
		}
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

// serverGetSlopeSurfaceY calculates the slope surface Y at the object's center X.
// Mirrors systems/collision.go:265-280.
func serverGetSlopeSurfaceY(object *resolv.Object, ramp *resolv.Object) float64 {
	playerCenterX := object.X + object.W/2
	relativeX := math.Max(0, math.Min(playerCenterX-ramp.X, ramp.W))
	slope := relativeX / ramp.W

	if ramp.HasTags(tagSlope45UpR) {
		return ramp.Y + ramp.H*(1-slope)
	}
	if ramp.HasTags(tagSlope45UpL) {
		return ramp.Y + ramp.H*slope
	}
	return ramp.Y
}

// serverSnapToSlope adjusts the player Y position to stay on a slope surface.
// Mirrors systems/collision.go:257-262.
func serverSnapToSlope(pp *PlayerPhysics, vel *netcomponents.NetVelocityData, ramp *resolv.Object) {
	surfaceY := serverGetSlopeSurfaceY(pp.Object, ramp)
	pp.Object.Y = surfaceY - pp.Object.H + slopeSurfaceOffset
	pp.OnGround = true
	vel.SpeedY = 0
}
