package systems

import (
	"math"

	"github.com/automoto/doomerang-mp/assets"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/network"
	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/automoto/doomerang-mp/tags"
	"github.com/solarlune/resolv"
)

// Client-side physics constants — must match server/core/physics.go exactly.
const (
	predGravity            = 0.75
	predJumpSpeed          = 15.0
	predMaxSpeed           = 6.0
	predAcceleration       = 0.75
	predFriction           = 0.5
	predMaxFallSpeed       = 10.0
	predMaxVertSpeed       = 16.0
	predSlopeSurfaceOffset = 0.1
)

// NetPrediction owns client-side prediction state for the local player.
type NetPrediction struct {
	Buffer *network.PredictionBuffer

	// Local physics state (mirrors server PlayerPhysics)
	VelX, VelY     float64
	OnGround       bool
	JumpWasPressed bool
	Initialized    bool // True after first server snapshot has been applied

	// Collision space for prediction
	Space     *resolv.Space
	PlayerObj *resolv.Object
}

// NewNetPrediction creates a new prediction system.
func NewNetPrediction() *NetPrediction {
	return &NetPrediction{
		Buffer:   &network.PredictionBuffer{},
		OnGround: true,
	}
}

// InitCollision builds a lightweight resolv.Space from the level's tile data
// for use in client-side prediction. This gives prediction real wall/ground/ramp
// collision instead of the GroundY approximation.
func (p *NetPrediction) InitCollision(tiles []assets.SolidTile, mapW, mapH int, spawnX, spawnY float64) {
	p.Space = resolv.NewSpace(mapW, mapH, 16, 16)

	for _, t := range tiles {
		var obj *resolv.Object
		switch t.SlopeType {
		case tags.Slope45UpRight:
			obj = resolv.NewObject(t.X, t.Y, t.Width, t.Height, tags.ResolvRamp, tags.Slope45UpRight)
		case tags.Slope45UpLeft:
			obj = resolv.NewObject(t.X, t.Y, t.Width, t.Height, tags.ResolvRamp, tags.Slope45UpLeft)
		default:
			obj = resolv.NewObject(t.X, t.Y, t.Width, t.Height, tags.ResolvSolid)
		}
		obj.SetShape(resolv.NewRectangle(0, 0, t.Width, t.Height))
		p.Space.Add(obj)
	}

	playerW := float64(cfg.Player.CollisionWidth)
	playerH := float64(cfg.Player.CollisionHeight)
	p.PlayerObj = resolv.NewObject(spawnX, spawnY, playerW, playerH, "player")
	p.PlayerObj.SetShape(resolv.NewRectangle(0, 0, playerW, playerH))
	p.Space.Add(p.PlayerObj)
}

// PredictStep applies one 60 Hz physics sub-step using the given input and
// updates the local player entity's NetPosition. It stores the result in the
// prediction buffer for later reconciliation.
func (p *NetPrediction) PredictStep(input messages.PlayerInput, pos *netcomponents.NetPositionData) {
	// --- Horizontal input ---
	if input.Direction != 0 {
		p.VelX += float64(input.Direction) * predAcceleration
	}

	// --- Jump (edge-triggered) ---
	jumpPressed := input.Actions[netconfig.ActionJump]
	if jumpPressed && !p.JumpWasPressed && p.OnGround {
		p.VelY = -predJumpSpeed
		p.OnGround = false
	}
	p.JumpWasPressed = jumpPressed

	// --- Friction (ground only) ---
	if p.OnGround {
		if p.VelX > predFriction {
			p.VelX -= predFriction
		} else if p.VelX < -predFriction {
			p.VelX += predFriction
		} else {
			p.VelX = 0
		}
	}

	// --- Clamp horizontal speed ---
	if p.VelX > predMaxSpeed {
		p.VelX = predMaxSpeed
	} else if p.VelX < -predMaxSpeed {
		p.VelX = -predMaxSpeed
	}

	// --- Gravity (unconditional — must match server/core/physics.go) ---
	p.VelY += predGravity
	if p.VelY > predMaxFallSpeed {
		p.VelY = predMaxFallSpeed
	}

	// --- Resolve collisions ---
	if p.PlayerObj != nil {
		p.PlayerObj.X = pos.X
		p.PlayerObj.Y = pos.Y
		p.PlayerObj.Update()
		p.resolveHorizontal()
		p.resolveVertical()
		pos.X = p.PlayerObj.X
		pos.Y = p.PlayerObj.Y
	} else {
		// Fallback: no collision space, bare movement
		pos.X += p.VelX
		dy := math.Max(math.Min(p.VelY, predMaxVertSpeed), -predMaxVertSpeed)
		pos.Y += dy
	}

	// Store prediction
	p.Buffer.Store(input, pos.X, pos.Y)
}

// resolveHorizontal handles horizontal movement with wall and ramp collision.
func (p *NetPrediction) resolveHorizontal() {
	dx := p.VelX
	if dx == 0 {
		return
	}

	// Check for ramp collision in front (walking uphill)
	if rampCheck := p.PlayerObj.Check(dx, 0, tags.ResolvRamp); rampCheck != nil {
		if ramps := rampCheck.ObjectsByTags(tags.ResolvRamp); len(ramps) > 0 {
			p.PlayerObj.X += dx
			p.PlayerObj.Update()
			p.predSnapToSlope(ramps[0])
			return
		}
	}
	// Check for ramp below (walking downhill)
	if rampCheck := p.PlayerObj.Check(dx, 1, tags.ResolvRamp); rampCheck != nil {
		if ramps := rampCheck.ObjectsByTags(tags.ResolvRamp); len(ramps) > 0 {
			p.PlayerObj.X += dx
			p.PlayerObj.Update()
			p.predSnapToSlope(ramps[0])
			return
		}
	}
	// Solid wall check
	if check := p.PlayerObj.Check(dx, 0, tags.ResolvSolid); check != nil {
		if solids := check.ObjectsByTags(tags.ResolvSolid); len(solids) > 0 {
			contact := check.ContactWithObject(solids[0])
			dx = contact.X()
			p.VelX = 0
		}
	}
	p.PlayerObj.X += dx
	p.PlayerObj.Update()
}

// resolveVertical handles vertical movement with ground, ceiling, and ramp collision.
func (p *NetPrediction) resolveVertical() {
	dy := p.VelY
	dy = math.Max(math.Min(dy, predMaxVertSpeed), -predMaxVertSpeed)

	checkDist := dy
	if dy >= 0 {
		checkDist++
	}

	if check := p.PlayerObj.Check(0, checkDist, tags.ResolvSolid, tags.ResolvRamp); check != nil {
		// Ramp landing (priority)
		if dy >= 0 {
			if ramps := check.ObjectsByTags(tags.ResolvRamp); len(ramps) > 0 {
				surfaceY := predGetSlopeSurfaceY(p.PlayerObj, ramps[0])
				playerBottom := p.PlayerObj.Y + p.PlayerObj.H
				if playerBottom+dy >= surfaceY {
					p.PlayerObj.Y = surfaceY - p.PlayerObj.H + predSlopeSurfaceOffset
					p.PlayerObj.Update()
					p.VelY = 0
					p.OnGround = true
					return
				}
			}
		}

		if solids := check.ObjectsByTags(tags.ResolvSolid); len(solids) > 0 {
			if dy >= 0 {
				contact := check.ContactWithObject(solids[0])
				p.PlayerObj.Y += contact.Y()
				p.PlayerObj.Update()
				p.VelY = 0
				p.OnGround = true
				return
			}
			// Hitting ceiling
			contact := check.ContactWithObject(solids[0])
			p.PlayerObj.Y += contact.Y()
			p.PlayerObj.Update()
			p.VelY = 0
			return
		}
	}

	// No collision — freefall
	p.OnGround = false
	p.PlayerObj.Y += dy
	p.PlayerObj.Update()
}

// predSnapToSlope adjusts the player Y position to stay on a slope surface.
func (p *NetPrediction) predSnapToSlope(ramp *resolv.Object) {
	surfaceY := predGetSlopeSurfaceY(p.PlayerObj, ramp)
	p.PlayerObj.Y = surfaceY - p.PlayerObj.H + predSlopeSurfaceOffset
	p.PlayerObj.Update()
	p.OnGround = true
	p.VelY = 0
}

// predGetSlopeSurfaceY calculates the slope surface Y at the object's center X.
// Mirrors systems/collision.go:265-280.
func predGetSlopeSurfaceY(object *resolv.Object, ramp *resolv.Object) float64 {
	playerCenterX := object.X + object.W/2
	relativeX := math.Max(0, math.Min(playerCenterX-ramp.X, ramp.W))
	slope := relativeX / ramp.W

	if ramp.HasTags(tags.Slope45UpRight) {
		return ramp.Y + ramp.H*(1-slope)
	}
	if ramp.HasTags(tags.Slope45UpLeft) {
		return ramp.Y + ramp.H*slope
	}
	return ramp.Y
}

