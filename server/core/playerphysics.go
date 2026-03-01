package core

import (
	"github.com/solarlune/resolv"
	"github.com/yohamta/donburi"
)

// PlayerPhysics holds per-player physics state on the server. This is not a
// donburi component — it exists only on the server and is never synced.
type PlayerPhysics struct {
	Object   *resolv.Object
	OnGround bool

	// Latest input snapshot (written by onPlayerInput, read by physics tick)
	Direction      int
	JumpPressed    bool
	JumpWasPressed bool // previous frame, for edge detection

	// Boomerang input state
	BoomerangPressed    bool
	BoomerangWasPressed bool
	MoveUpPressed       bool
	CrouchPressed       bool
	BoomerangCharging   bool
	BoomerangChargeTime int

	// Melee attack state
	AttackPressed    bool
	AttackWasPressed bool
	AttackFrame      int
	AttackIsPunch    bool // true = punch, false = kick (current attack)
	AttackIsJumpKick bool // true = aerial jump kick
	ComboStep        int  // 0 = next attack is punch, 1 = next attack is kick
	HitboxActive     bool
	HitTargets       map[donburi.Entity]struct{}
	InvulnFrames     int
	Dead             bool

	// State timer: counts down to unlock a locked animation state (Throw, Hit)
	LockedStateTimer int

	// Last processed input sequence (for client-side prediction reconciliation)
	LastInputSeq uint32
}

func newPlayerPhysics(level *ServerLevel, spawnX, spawnY float64) *PlayerPhysics {
	obj := resolv.NewObject(spawnX, spawnY, 16, 40, "player")
	obj.SetShape(resolv.NewRectangle(0, 0, 16, 40))
	level.Space.Add(obj)

	return &PlayerPhysics{
		Object:     obj,
		HitTargets: make(map[donburi.Entity]struct{}),
	}
}

func removePlayerPhysics(level *ServerLevel, pp *PlayerPhysics) {
	level.Space.Remove(pp.Object)
}
