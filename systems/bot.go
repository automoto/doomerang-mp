package systems

import (
	"math"

	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/tags"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

// UpdateBots generates input for bot-controlled players based on AI decisions.
// Must run BEFORE UpdateMultiPlayerInput to override bot inputs.
func UpdateBots(e *ecs.ECS) {
	// Collect all player positions for target selection
	var playerPositions []playerInfo
	tags.Player.Each(e.World, func(entry *donburi.Entry) {
		obj := components.Object.Get(entry)
		player := components.Player.Get(entry)
		health := components.Health.Get(entry)
		playerPositions = append(playerPositions, playerInfo{
			entry:       entry,
			playerIndex: player.PlayerIndex,
			x:           obj.X + obj.W/2,
			y:           obj.Y + obj.H/2,
			health:      health.Current,
			maxHealth:   health.Max,
			isBot:       entry.HasComponent(components.Bot),
		})
	})

	// Update each bot
	components.Bot.Each(e.World, func(entry *donburi.Entry) {
		updateBotAI(e, entry, playerPositions)
	})
}

type playerInfo struct {
	entry       *donburi.Entry
	playerIndex int
	x, y        float64
	health      int
	maxHealth   int
	isBot       bool
}

func updateBotAI(e *ecs.ECS, botEntry *donburi.Entry, players []playerInfo) {
	bot := components.Bot.Get(botEntry)
	input := components.PlayerInput.Get(botEntry)
	player := components.Player.Get(botEntry)
	obj := components.Object.Get(botEntry)
	health := components.Health.Get(botEntry)
	physics := components.Physics.Get(botEntry)

	// Decrement cooldowns
	if bot.DecisionTimer > 0 {
		bot.DecisionTimer--
	}
	if bot.AttackCooldown > 0 {
		bot.AttackCooldown--
	}
	if bot.JumpCooldown > 0 {
		bot.JumpCooldown--
	}
	if bot.DirectionTimer > 0 {
		bot.DirectionTimer--
	}

	// Clear previous inputs
	input.PreviousInput = input.CurrentInput
	input.CurrentInput = [cfg.ActionCount]bool{}

	botX := obj.X + obj.W/2
	botY := obj.Y + obj.H/2

	// Find nearest enemy player (not self, not on same team in team modes)
	target := findNearestTarget(bot, player.PlayerIndex, botX, botY, players)

	// Update target tracking
	if target != nil {
		bot.TargetPlayerIndex = target.playerIndex
		bot.TargetX = target.x
		bot.TargetY = target.y
		bot.DistanceToTarget = distance(botX, botY, target.x, target.y)
	} else {
		bot.TargetPlayerIndex = -1
		bot.DistanceToTarget = 9999
	}

	// Check health for retreat
	healthPercent := float64(health.Current) / float64(health.Max)

	// State machine with reaction delay
	if bot.DecisionTimer <= 0 {
		updateBotState(bot, target, healthPercent, physics)
		bot.DecisionTimer = bot.ReactionDelay / 3 // Periodic re-evaluation
	}

	// Generate inputs based on state
	generateBotInputs(bot, input, player, obj, physics, target, botX, botY)
}

func findNearestTarget(bot *components.BotData, myIndex int, myX, myY float64, players []playerInfo) *playerInfo {
	var nearest *playerInfo
	nearestDist := math.MaxFloat64

	for i := range players {
		p := &players[i]
		// Skip self
		if p.playerIndex == myIndex {
			continue
		}
		// Skip dead players
		if p.health <= 0 {
			continue
		}
		// In a real implementation, would also skip teammates

		dist := distance(myX, myY, p.x, p.y)
		if dist < nearestDist {
			nearestDist = dist
			nearest = p
		}
	}

	return nearest
}

func updateBotState(bot *components.BotData, target *playerInfo, healthPercent float64, physics *components.PhysicsData) {
	// Retreat if low health
	if healthPercent < bot.RetreatThreshold && target != nil {
		bot.AIState = components.BotStateRetreat
		return
	}

	// No target - patrol
	if target == nil {
		bot.AIState = components.BotStatePatrol
		return
	}

	// Target in attack range
	if bot.DistanceToTarget < bot.AttackRange {
		bot.AIState = components.BotStateAttack
		return
	}

	// Target in chase range
	if bot.DistanceToTarget < bot.ChaseRange {
		bot.AIState = components.BotStateChase
		return
	}

	// Default to patrol
	bot.AIState = components.BotStatePatrol
}

func generateBotInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, obj *components.ObjectData, physics *components.PhysicsData, target *playerInfo, botX, botY float64) {
	switch bot.AIState {
	case components.BotStatePatrol:
		generatePatrolInputs(bot, input, player, obj, physics)

	case components.BotStateChase:
		generateChaseInputs(bot, input, player, target, botX, botY, physics)

	case components.BotStateAttack:
		generateAttackInputs(bot, input, player, target, botX, botY, physics)

	case components.BotStateRetreat:
		generateRetreatInputs(bot, input, player, target, botX, botY, physics)

	case components.BotStateIdle:
		// Do nothing
	}
}

func generatePatrolInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, obj *components.ObjectData, physics *components.PhysicsData) {
	// Simple patrol - move in current direction, occasionally change
	if bot.DirectionTimer <= 0 {
		// Randomly change direction occasionally
		bot.DirectionTimer = 120 + int(player.Direction.X*60) // ~2-3 seconds
		// Could add randomness here, but keeping it deterministic for now
	}

	// Move in facing direction
	if player.Direction.X > 0 {
		input.CurrentInput[cfg.ActionMoveRight] = true
	} else {
		input.CurrentInput[cfg.ActionMoveLeft] = true
	}

	// Jump if stuck (no horizontal movement despite trying)
	if physics.OnGround != nil && math.Abs(physics.SpeedX) < 0.5 && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 30
		// Change direction after jump
		player.Direction.X = -player.Direction.X
	}
}

func generateChaseInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *playerInfo, botX, botY float64, physics *components.PhysicsData) {
	if target == nil {
		return
	}

	// Move toward target
	dx := target.x - botX
	dy := target.y - botY

	if dx > 10 {
		input.CurrentInput[cfg.ActionMoveRight] = true
		player.Direction.X = cfg.DirectionRight
	} else if dx < -10 {
		input.CurrentInput[cfg.ActionMoveLeft] = true
		player.Direction.X = cfg.DirectionLeft
	}

	// Jump if target is above or if stuck
	shouldJump := dy < -40 && physics.OnGround != nil && bot.JumpCooldown <= 0
	// Also jump if stuck
	if physics.OnGround != nil && math.Abs(physics.SpeedX) < 0.5 && math.Abs(dx) > 20 && bot.JumpCooldown <= 0 {
		shouldJump = true
	}

	if shouldJump {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 20
	}

	// Wall jump if wall sliding
	if physics.WallSliding != nil && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 15
	}
}

func generateAttackInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *playerInfo, botX, botY float64, physics *components.PhysicsData) {
	if target == nil {
		return
	}

	// Face target
	dx := target.x - botX
	if dx > 0 {
		player.Direction.X = cfg.DirectionRight
	} else {
		player.Direction.X = cfg.DirectionLeft
	}

	// Attack if cooldown is ready
	if bot.AttackCooldown <= 0 {
		input.CurrentInput[cfg.ActionAttack] = true
		bot.AttackCooldown = 30 + bot.ReactionDelay // Variable cooldown based on difficulty
	}

	// Slight movement toward target for better positioning
	if math.Abs(dx) > 20 {
		if dx > 0 {
			input.CurrentInput[cfg.ActionMoveRight] = true
		} else {
			input.CurrentInput[cfg.ActionMoveLeft] = true
		}
	}

	// Throw boomerang occasionally at medium range
	if bot.DistanceToTarget > 60 && bot.DistanceToTarget < 150 && bot.AttackCooldown <= 0 {
		input.CurrentInput[cfg.ActionBoomerang] = true
		bot.AttackCooldown = 60
	}
}

func generateRetreatInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *playerInfo, botX, botY float64, physics *components.PhysicsData) {
	if target == nil {
		return
	}

	// Move away from target
	dx := target.x - botX

	if dx > 0 {
		input.CurrentInput[cfg.ActionMoveLeft] = true
		player.Direction.X = cfg.DirectionLeft
	} else {
		input.CurrentInput[cfg.ActionMoveRight] = true
		player.Direction.X = cfg.DirectionRight
	}

	// Jump to escape
	if physics.OnGround != nil && bot.JumpCooldown <= 0 && bot.DistanceToTarget < 80 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 25
	}

	// Counter-attack if very close
	if bot.DistanceToTarget < 30 && bot.AttackCooldown <= 0 {
		input.CurrentInput[cfg.ActionAttack] = true
		bot.AttackCooldown = 45
	}
}

func distance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}
