package config

import "github.com/automoto/doomerang-mp/shared/netconfig"

// Type aliases — all existing client code using config.StateID etc. keeps working.
type StateID = netconfig.StateID
type MatchStateID = netconfig.MatchStateID

// Re-export match state constants.
const (
	MatchStateWaiting   = netconfig.MatchStateWaiting
	MatchStateCountdown = netconfig.MatchStateCountdown
	MatchStatePlaying   = netconfig.MatchStatePlaying
	MatchStateFinished  = netconfig.MatchStateFinished
)

// Re-export character/entity state constants.
const (
	StateNone = netconfig.StateNone

	Idle        = netconfig.Idle
	Crouch      = netconfig.Crouch
	Die         = netconfig.Die
	Guard       = netconfig.Guard
	GuardImpact = netconfig.GuardImpact
	Hit         = netconfig.Hit
	Jump        = netconfig.Jump
	Kick01      = netconfig.Kick01
	Kick02      = netconfig.Kick02
	Kick03      = netconfig.Kick03
	Knockback   = netconfig.Knockback
	Ledge       = netconfig.Ledge
	LedgeGrab   = netconfig.LedgeGrab
	Punch01     = netconfig.Punch01
	Punch02     = netconfig.Punch02
	Punch03     = netconfig.Punch03
	Running     = netconfig.Running
	Stunned     = netconfig.Stunned
	Throw       = netconfig.Throw
	Walk        = netconfig.Walk
	WallSlide   = netconfig.WallSlide

	StateAttackingPunch    = netconfig.StateAttackingPunch
	StateAttackingKick     = netconfig.StateAttackingKick
	StateChargingAttack    = netconfig.StateChargingAttack
	StateAttackingJump     = netconfig.StateAttackingJump
	StateChargingBoomerang = netconfig.StateChargingBoomerang

	StateSliding = netconfig.StateSliding

	StatePatrol       = netconfig.StatePatrol
	StateChase        = netconfig.StateChase
	StateApproachEdge = netconfig.StateApproachEdge

	StateJumpDust       = netconfig.StateJumpDust
	StateLandDust       = netconfig.StateLandDust
	StateSlideDust      = netconfig.StateSlideDust
	StateExplosionShort = netconfig.StateExplosionShort
	StatePlasma         = netconfig.StatePlasma
	StateGunshot        = netconfig.StateGunshot
	HitExplosion        = netconfig.HitExplosion
	ChargeUp            = netconfig.ChargeUp

	FirePulsing    = netconfig.FirePulsing
	FireContinuous = netconfig.FireContinuous
)

// Re-export the map (same reference, no copy).
var StateToFileName = netconfig.StateToFileName
