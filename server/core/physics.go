package core

import (
	"math"

	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
)

// Physics constants — matching config/config.go values used by the client.
const (
	gravity      = 0.75
	jumpSpeed    = 15.0
	maxSpeed     = 6.0
	acceleration = 0.75
	friction     = 0.5
	maxFallSpeed = 10.0
	maxVertSpeed = 16.0 // hard clamp from systems/collision.go:190
)

// updatePhysics runs sub-stepped physics for all players. Called once per
// server tick. Sub-stepping ensures the same physics constants that were tuned
// for 60 Hz work correctly at the server's lower tick rate.
func (s *Server) updatePhysics() {
	stepsPerTick := 60 / s.loop.tickRate // 3 at 20 Hz
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
	if dx != 0 {
		if check := pp.Object.Check(dx, 0, "solid"); check != nil {
			if solids := check.ObjectsByTags("solid"); len(solids) > 0 {
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

	if check := pp.Object.Check(0, checkDist, "solid"); check != nil {
		if solids := check.ObjectsByTags("solid"); len(solids) > 0 {
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
