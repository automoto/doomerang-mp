// Package netconfig defines lightweight types shared between client and server
// for network serialization. It must have zero dependencies on ebiten or any
// graphics library so the dedicated server binary stays headless.
package netconfig

// StateID identifies a character/entity state for animation and logic.
type StateID int

// MatchStateID represents the current state of a match.
type MatchStateID int

const (
	MatchStateWaiting   MatchStateID = iota // Waiting for players/setup
	MatchStateCountdown                     // Pre-match countdown (3, 2, 1)
	MatchStatePlaying                       // Active gameplay
	MatchStateFinished                      // Match over, showing results
)

const (
	StateNone StateID = -1

	// Character animation states
	Idle StateID = iota
	Crouch
	Die
	Guard
	GuardImpact
	Hit
	Jump
	Kick01
	Kick02
	Kick03
	Knockback
	Ledge
	LedgeGrab
	Punch01
	Punch02
	Punch03
	Running
	Stunned
	Throw
	Walk
	WallSlide

	// Combat specific states
	StateAttackingPunch
	StateAttackingKick
	StateChargingAttack
	StateAttackingJump
	StateChargingBoomerang

	// Movement states
	StateSliding

	// Enemy AI states
	StatePatrol
	StateChase
	StateApproachEdge

	// VFX states (dust and impact effects)
	StateJumpDust
	StateLandDust
	StateSlideDust
	StateExplosionShort
	StatePlasma
	StateGunshot
	HitExplosion
	ChargeUp

	// Fire obstacle states
	FirePulsing
	FireContinuous
)

// StateToFileName maps StateID to the corresponding filename prefix.
var StateToFileName = map[StateID]string{
	Idle:        "idle",
	Crouch:      "crouch",
	Die:         "die",
	Guard:       "guard",
	GuardImpact: "guardimpact",
	Hit:         "hit",
	Jump:        "jump",
	Kick01:      "kick01",
	Kick02:      "kick02",
	Kick03:      "kick03",
	Knockback:   "knockback",
	Ledge:       "ledge",
	LedgeGrab:   "ledgegrab",
	Punch01:     "punch01",
	Punch02:     "punch02",
	Punch03:     "punch03",
	Running:     "running",
	Stunned:     "stunned",
	Throw:       "throw",
	Walk:        "walk",
	WallSlide:   "wallslide",

	StateAttackingPunch:    "punch01",
	StateAttackingKick:     "kick01",
	StateAttackingJump:     "kick02",
	StateChargingAttack:    "idle",
	StateChargingBoomerang: "throw",

	StateSliding: "slide",

	StatePatrol:       "walk",
	StateChase:        "running",
	StateApproachEdge: "walk",

	StateJumpDust:       "jumpdust",
	StateLandDust:       "landingdust",
	StateSlideDust:      "slidedust",
	StateExplosionShort: "explosion_short",
	StatePlasma:         "plasma",
	StateGunshot:        "gunshot_rifle",
	HitExplosion:        "explosion",
	ChargeUp:            "level_up",

	FirePulsing:    "flames_pulse",
	FireContinuous: "flame_continuous",
}

func (s StateID) String() string {
	if name, ok := StateToFileName[s]; ok {
		return name
	}
	return "unknown"
}

// Boomerang state constants
const (
	BoomerangOutbound = 0
	BoomerangInbound  = 1
)

// ActionID represents a logical game action.
type ActionID int

const (
	ActionNone ActionID = iota
	ActionMoveLeft
	ActionMoveRight
	ActionMoveUp
	ActionJump
	ActionAttack
	ActionCrouch
	ActionBoomerang
	ActionPause
	ActionMenuUp
	ActionMenuDown
	ActionMenuLeft
	ActionMenuRight
	ActionMenuSelect
	ActionMenuBack
	ActionCount // Must be last - used for array sizing
)
