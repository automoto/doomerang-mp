package core

import "github.com/solarlune/resolv"

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
		Object: obj,
	}
}

func removePlayerPhysics(level *ServerLevel, pp *PlayerPhysics) {
	level.Space.Remove(pp.Object)
}
