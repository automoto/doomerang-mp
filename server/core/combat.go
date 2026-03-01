package core

import (
	"log"
	"time"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/leap-fish/necs/esync"
	"github.com/yohamta/donburi"
)

// Melee hitbox active window (in ticks at server tick rate).
const (
	meleeHitboxStart     = 3
	meleeHitboxEnd       = 8
	jumpKickHitboxStart  = 2
	jumpKickHitboxEnd    = 12 // Jump kick has a longer active window (2x)
)

// updateCombat is called once per server tick, after physics.
func (s *Server) updateCombat() {
	// Only run combat during active gameplay
	if s.match.State != netcomponents.MatchStatePlaying {
		return
	}

	for entity, pp := range s.playerPhysics {
		if !s.world.Valid(entity) {
			continue
		}
		if pp.Dead {
			continue
		}

		// Decrement invulnerability frames
		if pp.InvulnFrames > 0 {
			pp.InvulnFrames--
		}

		s.processMeleeAttack(entity, pp)
	}
}

// processMeleeAttack edge-detects the attack button, manages the attack frame
// counter and hitbox window, and checks for hits. Alternates punch/kick via
// ComboStep on ground; uses jump kick when airborne.
func (s *Server) processMeleeAttack(entity donburi.Entity, pp *PlayerPhysics) {
	// Edge detect: new press → start attack
	if pp.AttackPressed && !pp.AttackWasPressed && pp.AttackFrame == 0 {
		pp.AttackFrame = 1
		pp.HitboxActive = false
		clear(pp.HitTargets)

		entry := s.world.Entry(entity)

		if !pp.OnGround {
			// Airborne → jump kick (always kick config, no combo alternation)
			pp.AttackIsJumpKick = true
			pp.AttackIsPunch = false

			if entry.HasComponent(netcomponents.NetPlayerState) {
				state := netcomponents.NetPlayerState.Get(entry)
				state.StateID = netconfig.StateAttackingJump
			}
			pp.LockedStateTimer = 14 // Longer lock for jump kick
		} else {
			// Grounded → alternate punch/kick based on ComboStep
			pp.AttackIsJumpKick = false
			pp.AttackIsPunch = pp.ComboStep == 0
			if pp.ComboStep == 0 {
				pp.ComboStep = 1
			} else {
				pp.ComboStep = 0
			}

			if entry.HasComponent(netcomponents.NetPlayerState) {
				state := netcomponents.NetPlayerState.Get(entry)
				if pp.AttackIsPunch {
					state.StateID = netconfig.StateAttackingPunch
				} else {
					state.StateID = netconfig.StateAttackingKick
				}
			}
			pp.LockedStateTimer = 10
		}

		// Broadcast attack initiation event for SFX
		var attackerNetID uint
		if nid := esync.GetNetworkId(entry); nid != nil {
			attackerNetID = uint(*nid)
		}
		s.broadcastEvent(messages.MeleeAttackEvent{
			AttackerNetworkID: attackerNetID,
			IsPunch:           pp.AttackIsPunch,
		})
	}
	pp.AttackWasPressed = pp.AttackPressed

	if pp.AttackFrame == 0 {
		return
	}

	// Advance attack frame
	pp.AttackFrame++

	// Hitbox active window (jump kick has a wider window)
	hitStart := meleeHitboxStart
	hitEnd := meleeHitboxEnd
	if pp.AttackIsJumpKick {
		hitStart = jumpKickHitboxStart
		hitEnd = jumpKickHitboxEnd
	}

	if pp.AttackFrame >= hitStart && pp.AttackFrame <= hitEnd {
		pp.HitboxActive = true
		s.checkMeleeHitbox(entity, pp)
	} else {
		pp.HitboxActive = false
	}

	// End attack after hitbox window + a short buffer
	if pp.AttackFrame > hitEnd+2 {
		pp.AttackFrame = 0
		pp.HitboxActive = false
		pp.AttackIsJumpKick = false
		// Reset combo when attack sequence ends (mirrors offline transitionToMovementState)
		pp.ComboStep = 0
	}
}

// checkMeleeHitbox builds an AABB in front of the attacker and checks overlap
// with all other player collision rects.
func (s *Server) checkMeleeHitbox(attackerEntity donburi.Entity, attackerPP *PlayerPhysics) {
	entry := s.world.Entry(attackerEntity)
	if !entry.HasComponent(netcomponents.NetPlayerState) {
		return
	}
	state := netcomponents.NetPlayerState.Get(entry)

	// Build hitbox rect in front of the attacker
	dir := 1.0
	if state.Direction < 0 {
		dir = -1.0
	}

	// Use per-attack hitbox dimensions
	var hitW, hitH float64
	if attackerPP.AttackIsPunch {
		hitW = cfg.Combat.PunchHitboxWidth
		hitH = cfg.Combat.PunchHitboxHeight
	} else {
		hitW = cfg.Combat.KickHitboxWidth
		hitH = cfg.Combat.KickHitboxHeight
	}

	// Hitbox origin: in front of player, vertically centered on player
	var hitX float64
	if dir > 0 {
		hitX = attackerPP.Object.X + attackerPP.Object.W
	} else {
		hitX = attackerPP.Object.X - hitW
	}
	hitY := attackerPP.Object.Y + (attackerPP.Object.H-hitH)/2

	// Check against all other players
	for targetEntity, targetPP := range s.playerPhysics {
		if targetEntity == attackerEntity {
			continue
		}
		if !s.world.Valid(targetEntity) {
			continue
		}
		if targetPP.Dead {
			continue
		}
		if targetPP.InvulnFrames > 0 {
			continue
		}
		if _, already := attackerPP.HitTargets[targetEntity]; already {
			continue
		}

		// AABB overlap test
		tX := targetPP.Object.X
		tY := targetPP.Object.Y
		tW := targetPP.Object.W
		tH := targetPP.Object.H

		if hitX < tX+tW && hitX+hitW > tX && hitY < tY+tH && hitY+hitH > tY {
			s.applyMeleeHit(attackerEntity, attackerPP, targetEntity, targetPP, hitX+hitW/2, hitY+hitH/2)
		}
	}
}

// applyMeleeHit applies damage, knockback, and state changes — mirrors boomerang.go:hitPlayer().
func (s *Server) applyMeleeHit(
	attackerEntity donburi.Entity, attackerPP *PlayerPhysics,
	targetEntity donburi.Entity, targetPP *PlayerPhysics,
	hitX, hitY float64,
) {
	attackerPP.HitTargets[targetEntity] = struct{}{}

	targetEntry := s.world.Entry(targetEntity)

	// Select per-attack damage and knockback
	var damage int
	var knockbackForce float64
	if attackerPP.AttackIsPunch {
		damage = cfg.Combat.PlayerPunchDamage
		knockbackForce = cfg.Combat.PlayerPunchKnockback
	} else {
		damage = cfg.Combat.PlayerKickDamage
		knockbackForce = cfg.Combat.PlayerKickKnockback
	}

	// Apply damage
	if targetEntry.HasComponent(netcomponents.NetPlayerState) {
		state := netcomponents.NetPlayerState.Get(targetEntry)
		state.Health -= damage
		if state.Health < 0 {
			state.Health = 0
		}
		state.StateID = netconfig.Hit
	}

	// Lock hit state animation
	targetPP.LockedStateTimer = 10
	targetPP.InvulnFrames = cfg.Combat.PlayerInvulnFrames

	// Apply knockback
	attackerEntry := s.world.Entry(attackerEntity)
	knockDir := 1.0
	if attackerEntry.HasComponent(netcomponents.NetPlayerState) {
		aState := netcomponents.NetPlayerState.Get(attackerEntry)
		if aState.Direction < 0 {
			knockDir = -1.0
		}
	}
	knockX := knockDir * knockbackForce
	knockY := cfg.Combat.KnockbackUpwardForce

	if targetEntry.HasComponent(netcomponents.NetVelocity) {
		vel := netcomponents.NetVelocity.Get(targetEntry)
		vel.SpeedX += knockX
		vel.SpeedY += knockY
	}

	// Get network IDs for event broadcast
	var attackerNetID, targetNetID uint
	if nid := esync.GetNetworkId(attackerEntry); nid != nil {
		attackerNetID = uint(*nid)
	}
	if nid := esync.GetNetworkId(targetEntry); nid != nil {
		targetNetID = uint(*nid)
	}

	s.broadcastEvent(messages.MeleeHitEvent{
		AttackerNetworkID: attackerNetID,
		TargetNetworkID:   targetNetID,
		HitX:              hitX,
		HitY:              hitY,
		Damage:            damage,
		KnockbackX:        knockX,
		KnockbackY:        knockY,
	})

	// Check for death
	if targetEntry.HasComponent(netcomponents.NetPlayerState) {
		state := netcomponents.NetPlayerState.Get(targetEntry)
		if state.Health <= 0 {
			s.handlePlayerDeath(targetEntity, targetPP, attackerNetID)
		}
	}
}

// handlePlayerDeath sets the Die state, decrements lives, and either schedules
// respawn or marks the player as eliminated.
func (s *Server) handlePlayerDeath(entity donburi.Entity, pp *PlayerPhysics, killerNetID uint) {
	pp.Dead = true

	entry := s.world.Entry(entity)
	if entry.HasComponent(netcomponents.NetPlayerState) {
		state := netcomponents.NetPlayerState.Get(entry)
		state.StateID = netconfig.Die
	}
	pp.LockedStateTimer = cfg.Match.RespawnDelay // Keep Die state locked until respawn

	var victimNetID uint
	if nid := esync.GetNetworkId(entry); nid != nil {
		victimNetID = uint(*nid)
	}

	s.broadcastEvent(messages.DeathEvent{
		VictimID: victimNetID,
		KillerID: killerNetID,
	})

	// Record in match system
	s.match.AddDeath(uint32(victimNetID))
	if killerNetID != 0 && killerNetID != victimNetID {
		s.match.AddKO(uint32(killerNetID))
	}

	// Decrement lives
	nid32 := uint32(victimNetID)
	s.match.Lives[nid32]--

	if entry.HasComponent(netcomponents.NetPlayerState) {
		netcomponents.NetPlayerState.Get(entry).Lives = s.match.Lives[nid32]
	}

	if s.match.Lives[nid32] <= 0 {
		// Player is eliminated — no respawn
		s.match.Lives[nid32] = 0
		s.match.Eliminated[nid32] = true

		s.broadcastEvent(messages.MatchEvent{
			Type:        "player_eliminated",
			PlayerID:    nid32,
			RoundNumber: s.match.CurrentRound,
		})

		s.match.checkRoundEndCondition()
		return
	}

	// Still has lives — schedule respawn
	respawnMs := time.Duration(cfg.Match.RespawnDelay) * time.Second / 60
	time.AfterFunc(respawnMs, func() {
		s.cmdCh <- func() {
			s.respawnPlayer(entity)
		}
	})
}

// respawnPlayer resets a dead player to a spawn point with full health.
func (s *Server) respawnPlayer(entity donburi.Entity) {
	if !s.world.Valid(entity) {
		return
	}
	pp, ok := s.playerPhysics[entity]
	if !ok {
		return
	}

	// Pick a spawn point
	spawnX, spawnY := 100.0, 100.0
	if len(s.activeLevel.SpawnPoints) > 0 {
		idx := int(entity) % len(s.activeLevel.SpawnPoints)
		sp := s.activeLevel.SpawnPoints[idx]
		spawnX, spawnY = sp.X, sp.Y
	}

	// Reset physics
	pp.Object.X = spawnX
	pp.Object.Y = spawnY
	pp.Object.Update()
	pp.Dead = false
	pp.AttackFrame = 0
	pp.HitboxActive = false
	pp.ComboStep = 0
	pp.InvulnFrames = cfg.Player.RespawnInvulnFrames
	pp.LockedStateTimer = 0

	entry := s.world.Entry(entity)

	var playerNetID32 uint32
	if nid := esync.GetNetworkId(entry); nid != nil {
		playerNetID32 = uint32(*nid)
	}

	// Reset net components
	if entry.HasComponent(netcomponents.NetPosition) {
		pos := netcomponents.NetPosition.Get(entry)
		pos.X = spawnX
		pos.Y = spawnY
	}
	if entry.HasComponent(netcomponents.NetVelocity) {
		vel := netcomponents.NetVelocity.Get(entry)
		vel.SpeedX = 0
		vel.SpeedY = 0
	}
	if entry.HasComponent(netcomponents.NetPlayerState) {
		state := netcomponents.NetPlayerState.Get(entry)
		state.Health = cfg.Player.Health
		state.StateID = netconfig.Idle
		state.Lives = s.match.Lives[playerNetID32]
	}

	s.broadcastEvent(messages.RespawnEvent{
		PlayerNetworkID: uint(playerNetID32),
		X:               spawnX,
		Y:               spawnY,
	})

	log.Printf("Player networkID=%d respawned at (%.0f, %.0f)", playerNetID32, spawnX, spawnY)
}

// TODO: Add lag compensation for melee hitbox checks. Melee range is short,
// so at typical latencies the AABB check is close enough, but for competitive
// play we may want to rewind target positions by the attacker's RTT/2.
