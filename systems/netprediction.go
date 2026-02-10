package systems

import (
	"math"

	"github.com/automoto/doomerang-mp/assets"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/network"
	"github.com/automoto/doomerang-mp/shared/gamemath"
	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/automoto/doomerang-mp/tags"
	"github.com/solarlune/resolv"
)

// NetPrediction owns client-side prediction state for the local player.
type NetPrediction struct {
	Buffer *network.PredictionBuffer

	// Local physics state (mirrors server PlayerPhysics)
	VelX, VelY     float64
	OnGround       bool
	WasOnGround    bool // Previous frame's ground state for transition detection
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
	wasOnGround := p.OnGround

	// Skip acceleration during charging — friction only, matching offline
	if input.Direction != 0 && !input.Actions[netconfig.ActionBoomerang] {
		p.VelX += float64(input.Direction) * cfg.Player.Acceleration
	}

	jumpPressed := input.Actions[netconfig.ActionJump]
	if jumpPressed && !p.JumpWasPressed && p.OnGround {
		p.VelY = -cfg.Player.JumpSpeed
		p.OnGround = false
	}
	p.JumpWasPressed = jumpPressed

	if p.OnGround {
		p.VelX = gamemath.ApplyFriction(p.VelX, cfg.Player.Friction)
	}

	p.VelX = gamemath.ClampSpeed(p.VelX, cfg.Player.MaxSpeed)

	// Must match server/core/physics.go
	p.VelY += cfg.Physics.Gravity
	if p.VelY > cfg.Physics.MaxFallSpeed {
		p.VelY = cfg.Physics.MaxFallSpeed
	}

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
		dy := math.Max(math.Min(p.VelY, cfg.Physics.MaxVertSpeed), -cfg.Physics.MaxVertSpeed)
		pos.Y += dy
	}

	p.WasOnGround = wasOnGround
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
	dy = math.Max(math.Min(dy, cfg.Physics.MaxVertSpeed), -cfg.Physics.MaxVertSpeed)

	checkDist := dy
	if dy >= 0 {
		checkDist++
	}

	if check := p.PlayerObj.Check(0, checkDist, tags.ResolvSolid, tags.ResolvRamp); check != nil {
		// Ramp landing (priority)
		if dy >= 0 {
			if ramps := check.ObjectsByTags(tags.ResolvRamp); len(ramps) > 0 {
				surfaceY := gamemath.GetSlopeSurfaceY(p.PlayerObj, ramps[0], tags.Slope45UpRight, tags.Slope45UpLeft)
				playerBottom := p.PlayerObj.Y + p.PlayerObj.H
				if playerBottom+dy >= surfaceY {
					p.PlayerObj.Y = gamemath.SnapToSlopeY(p.PlayerObj.H, surfaceY, cfg.Physics.SlopeSurfaceOffset)
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
	surfaceY := gamemath.GetSlopeSurfaceY(p.PlayerObj, ramp, tags.Slope45UpRight, tags.Slope45UpLeft)
	p.PlayerObj.Y = gamemath.SnapToSlopeY(p.PlayerObj.H, surfaceY, cfg.Physics.SlopeSurfaceOffset)
	p.PlayerObj.Update()
	p.OnGround = true
	p.VelY = 0
}
