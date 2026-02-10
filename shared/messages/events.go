package messages

// HitEvent is broadcast when an attack connects
type HitEvent struct {
	AttackerID uint // NetworkId of attacker
	TargetID   uint // NetworkId of target
	Damage     int
	KnockbackX float64
	KnockbackY float64
}

// DeathEvent is broadcast when a player/enemy dies
type DeathEvent struct {
	VictimID uint // NetworkId of victim
	KillerID uint // NetworkId of killer (0 if environmental)
}

// SpawnEvent is broadcast when a new entity spawns
type SpawnEvent struct {
	NetworkID  uint   // Assigned NetworkId
	EntityType string // "player", "enemy", "boomerang"
	X, Y       float64
}

// DespawnEvent is broadcast when an entity is removed
type DespawnEvent struct {
	NetworkID uint
}

// BoomerangChargeEvent is sent when a player begins charging a boomerang throw
type BoomerangChargeEvent struct {
	OwnerNetworkID uint
	X, Y           float64 // Player feet position for VFX
}

// BoomerangThrowEvent is sent when a player throws a boomerang
type BoomerangThrowEvent struct {
	OwnerNetworkID uint
	X, Y           float64
	DirectionX     float64 // Normalized direction
	DirectionY     float64
	ChargeLevel    float64 // 0.0 to 1.0
}

// BoomerangCatchEvent is sent when a boomerang returns to owner
type BoomerangCatchEvent struct {
	OwnerNetworkID uint
}

// BoomerangHitEvent is sent when a boomerang hits a player
type BoomerangHitEvent struct {
	AttackerNetworkID   uint
	TargetNetworkID     uint
	HitX, HitY         float64
	ChargeRatio         float64
	Damage              int
	KnockbackX          float64
	KnockbackY          float64
}

// MatchStateChangeEvent is broadcast when match state changes
type MatchStateChangeEvent struct {
	NewState int     // MatchState enum value
	Timer    float64 // Countdown or remaining time
}

// ScoreUpdateEvent is broadcast when scores change
type ScoreUpdateEvent struct {
	Scores map[uint]int // NetworkId -> score
}
