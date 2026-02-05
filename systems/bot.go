package systems

import (
	"math"
	"math/rand"

	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/tags"
	"github.com/solarlune/resolv"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

// Random number generator for bot decision making.
// Uses fixed seed for deterministic replay support.
var rng = rand.New(rand.NewSource(42))

// Package-level nav grid cache (created once per level).
// Note: This is safe in single-threaded game loop. The cache is
// invalidated when level changes (checked via navGridLevelID).
var cachedNavGrid *NavGrid
var navGridLevelID string

// UpdateBots generates input for bot-controlled players based on AI decisions.
// Must run BEFORE UpdateMultiPlayerInput to override bot inputs.
func UpdateBots(e *ecs.ECS) {
	// Only run bot AI when match is actively playing
	if !IsMatchPlaying(e) {
		return
	}

	// Get space for line-of-sight checks and gap detection
	var space *resolv.Space
	if spaceEntry, ok := components.Space.First(e.World); ok {
		space = components.Space.Get(spaceEntry)
	}

	// Get nav grid (create once and cache per level)
	navGrid := getOrCreateNavGrid(e, space)

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
		updateBotAI(e, entry, playerPositions, space, navGrid)
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

func updateBotAI(e *ecs.ECS, botEntry *donburi.Entry, players []playerInfo, space *resolv.Space, navGrid *NavGrid) {
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

	// PRIORITY 1: Check for incoming threats and react defensively
	threat := detectIncomingThreats(e, botX, botY, player.PlayerIndex)
	if generateDefensiveInputs(bot, input, player, threat, botX, botY, physics) {
		return // Defensive action takes priority
	}

	// Check health for retreat
	healthPercent := float64(health.Current) / float64(health.Max)

	// State machine with reaction delay
	if bot.DecisionTimer <= 0 {
		updateBotState(bot, target, healthPercent, physics)
		bot.DecisionTimer = bot.ReactionDelay / 3 // Periodic re-evaluation
	}

	// Generate inputs based on state
	generateBotInputs(bot, input, player, obj, physics, target, botX, botY, space, navGrid)
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

	// No target - idle (shouldn't happen in PvP)
	if target == nil {
		bot.AIState = components.BotStateIdle
		return
	}

	// Target in attack range
	if bot.DistanceToTarget < bot.AttackRange {
		bot.AIState = components.BotStateAttack
		return
	}

	// ALWAYS CHASE - no more patrol
	bot.AIState = components.BotStateChase
}

func generateBotInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, obj *components.ObjectData, physics *components.PhysicsData, target *playerInfo, botX, botY float64, space *resolv.Space, navGrid *NavGrid) {
	switch bot.AIState {
	case components.BotStateChase:
		generateChaseInputs(bot, input, player, target, botX, botY, obj, physics, space, navGrid)

	case components.BotStateAttack:
		generateAttackInputs(bot, input, player, target, botX, botY, physics, space)

	case components.BotStateRetreat:
		generateRetreatInputs(bot, input, player, target, botX, botY, obj, physics, space)

	case components.BotStateIdle:
		// Do nothing
	}
}

func generateChaseInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *playerInfo, botX, botY float64, obj *components.ObjectData, physics *components.PhysicsData, space *resolv.Space, navGrid *NavGrid) {
	if target == nil {
		return
	}

	dx := target.x - botX
	dy := target.y - botY

	// Determine movement direction
	movingRight := dx > 0
	if dx > 10 {
		input.CurrentInput[cfg.ActionMoveRight] = true
		player.Direction.X = cfg.DirectionRight
	} else if dx < -10 {
		input.CurrentInput[cfg.ActionMoveLeft] = true
		player.Direction.X = cfg.DirectionLeft
	}

	// Gap detection - jump over gaps when on ground
	if physics.OnGround != nil && bot.JumpCooldown <= 0 {
		gapDetected, gapWidth := detectGapAhead(space, obj, movingRight)
		if gapDetected && gapWidth < 240 { // Max jump distance is ~240 pixels
			input.CurrentInput[cfg.ActionJump] = true
			bot.JumpCooldown = 20 // Short cooldown for gap jumps
		}
	}

	// Jump if target is significantly above us
	if dy < -60 && physics.OnGround != nil && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 45
	}

	// Wall jump if wall sliding
	if physics.WallSliding != nil && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 30
	}
}

func generateAttackInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *playerInfo, botX, botY float64, physics *components.PhysicsData, space *resolv.Space) {
	if target == nil {
		return
	}

	// Face target
	dx := target.x - botX
	dy := target.y - botY
	if dx > 0 {
		player.Direction.X = cfg.DirectionRight
	} else {
		player.Direction.X = cfg.DirectionLeft
	}

	dist := bot.DistanceToTarget

	// Always move toward target if not in melee range (keep pressure on)
	if dist > 40 {
		if dx > 0 {
			input.CurrentInput[cfg.ActionMoveRight] = true
		} else {
			input.CurrentInput[cfg.ActionMoveLeft] = true
		}
	}

	// Attack cooldown check - can't attack but keep moving
	if bot.AttackCooldown > 0 {
		return
	}

	// Check line of sight for ranged attacks
	hasLOS := hasLineOfSight(space, botX, botY, target.x, target.y)
	onGround := physics.OnGround != nil

	// Choose attack based on situation
	attack := chooseAttack(bot, dist, dy, hasLOS, onGround)

	switch attack {
	case AttackPunch:
		input.CurrentInput[cfg.ActionAttack] = true
		bot.AttackCooldown = 20 + bot.ReactionDelay/2

	case AttackJumpKick:
		if bot.JumpCooldown <= 0 {
			input.CurrentInput[cfg.ActionJump] = true
			input.CurrentInput[cfg.ActionAttack] = true
			bot.JumpCooldown = 50 // Increased cooldown
			bot.AttackCooldown = 25
		}

	case AttackBoomerang:
		// Aim up/down based on target position
		if dy < -30 {
			input.CurrentInput[cfg.ActionMoveUp] = true
		}
		input.CurrentInput[cfg.ActionBoomerang] = true
		bot.AttackCooldown = 90

	case AttackApproach:
		// Already moving above, nothing extra needed
	}
}

func generateRetreatInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *playerInfo, botX, botY float64, obj *components.ObjectData, physics *components.PhysicsData, space *resolv.Space) {
	if target == nil {
		return
	}

	dx := target.x - botX
	dist := bot.DistanceToTarget

	// Face target while retreating (to throw boomerangs and counter-attack)
	if dx > 0 {
		player.Direction.X = cfg.DirectionRight
	} else {
		player.Direction.X = cfg.DirectionLeft
	}

	// Move away from target but not constantly - create some space then fight
	movingRight := dx < 0 // Moving away from target
	if dist < 120 {
		if dx > 0 {
			input.CurrentInput[cfg.ActionMoveLeft] = true
		} else {
			input.CurrentInput[cfg.ActionMoveRight] = true
		}

		// Gap detection while retreating
		if physics.OnGround != nil && bot.JumpCooldown <= 0 {
			gapDetected, gapWidth := detectGapAhead(space, obj, movingRight)
			if gapDetected && gapWidth < 240 {
				input.CurrentInput[cfg.ActionJump] = true
				bot.JumpCooldown = 20
			}
		}
	}

	// Jump to escape if very close
	if physics.OnGround != nil && bot.JumpCooldown <= 0 && dist < 40 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 60
	}

	// Be aggressive even while retreating!
	if bot.AttackCooldown <= 0 {
		// Throw boomerang at medium range - good for retreating
		if dist > 60 && dist < 250 {
			input.CurrentInput[cfg.ActionBoomerang] = true
			bot.AttackCooldown = 70
		} else if dist < 50 {
			// Counter-attack if close - punch them away
			input.CurrentInput[cfg.ActionAttack] = true
			bot.AttackCooldown = 30
		}
	}
}

func distance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// detectGapAhead checks if there's a gap in front of the bot
// Returns (gapDetected, gapWidth) - gapWidth is approximate distance to next ground
func detectGapAhead(space *resolv.Space, obj *components.ObjectData, movingRight bool) (bool, float64) {
	if space == nil {
		return false, 0
	}

	// Use config values for gap detection
	checkDist := cfg.Pathfinding.GapCheckDist
	checkDepth := cfg.Pathfinding.GapCheckDepth
	maxScanDist := cfg.Pathfinding.MaxJumpDistance
	scanStep := cfg.Pathfinding.LOSStepSize

	// Start position - bottom edge of bot, ahead in movement direction
	startX := obj.X + obj.W/2
	startY := obj.Y + obj.H // Bottom of bot

	if movingRight {
		startX = obj.X + obj.W + checkDist
	} else {
		startX = obj.X - checkDist
	}

	// Check if there's ground directly below the check point
	hasGroundBelow := false
	for _, checkObj := range space.Objects() {
		if !checkObj.HasTags(tags.ResolvSolid) {
			continue
		}

		// Check if this object is below our check point
		if startX >= checkObj.X && startX <= checkObj.X+checkObj.W &&
			checkObj.Y >= startY && checkObj.Y <= startY+checkDepth {
			hasGroundBelow = true
			break
		}
	}

	if hasGroundBelow {
		return false, 0 // No gap, ground exists ahead
	}

	// Gap detected - now measure how wide it is
	direction := 1.0
	if !movingRight {
		direction = -1.0
	}

	for dist := checkDist; dist < maxScanDist; dist += scanStep {
		scanX := obj.X + obj.W/2 + direction*dist

		for _, checkObj := range space.Objects() {
			if !checkObj.HasTags(tags.ResolvSolid) {
				continue
			}

			// Check if this object provides ground at scan position
			if scanX >= checkObj.X && scanX <= checkObj.X+checkObj.W &&
				checkObj.Y >= startY && checkObj.Y <= startY+checkDepth*2 {
				// Found ground - return gap width
				return true, dist - checkDist
			}
		}
	}

	// Very wide gap (wider than max jump) - still report it
	return true, maxScanDist
}

// hasLineOfSight checks if there's a clear path between two points
// Uses manual AABB intersection to avoid modifying the physics space
func hasLineOfSight(space *resolv.Space, x1, y1, x2, y2 float64) bool {
	if space == nil {
		return true // Assume clear if no space available
	}

	dx := x2 - x1
	dy := y2 - y1
	dist := math.Sqrt(dx*dx + dy*dy)

	if dist == 0 {
		return true
	}

	// Normalize direction
	dx /= dist
	dy /= dist

	// Use config values for LOS checks
	stepSize := cfg.Pathfinding.LOSStepSize
	checkSize := cfg.Pathfinding.LOSCheckSize

	for d := stepSize; d < dist-stepSize; d += stepSize {
		checkX := x1 + dx*d
		checkY := y1 + dy*d

		// Check against all objects with ResolvSolid tag using manual AABB
		for _, obj := range space.Objects() {
			if !obj.HasTags(tags.ResolvSolid) {
				continue
			}

			// Simple AABB intersection check
			if checkX+checkSize > obj.X && checkX-checkSize < obj.X+obj.W &&
				checkY+checkSize > obj.Y && checkY-checkSize < obj.Y+obj.H {
				return false // Blocked
			}
		}
	}

	return true // Clear line of sight
}

// AttackChoice represents possible attack options
type AttackChoice int

const (
	AttackPunch AttackChoice = iota
	AttackJumpKick
	AttackBoomerang
	AttackApproach
)

// Pre-allocated buffers for chooseAttack to avoid allocations in hot path
var (
	attackOptions = make([]AttackChoice, 0, 4)
	attackWeights = make([]float64, 0, 4)
)

// chooseAttack selects an attack based on situation with weighted randomness
func chooseAttack(bot *components.BotData, dist float64, dy float64, hasLOS bool, onGround bool) AttackChoice {
	// Reset pre-allocated slices
	options := attackOptions[:0]
	weights := attackWeights[:0]

	// Use config values for attack thresholds
	combat := &cfg.BotCombat

	// Punch always viable at close range - high weight
	if dist < combat.PunchRange {
		options = append(options, AttackPunch)
		weights = append(weights, 5.0)
	}

	// Jump kick only occasionally and at specific range
	if dist < combat.JumpKickMaxRange && dist > combat.JumpKickMinRange && onGround {
		options = append(options, AttackJumpKick)
		weights = append(weights, 0.5) // Low weight - rare jump kicks
	}

	// Boomerang at medium-long range with LOS
	if dist > combat.BoomerangMinRange && dist < combat.BoomerangMaxRange && hasLOS {
		options = append(options, AttackBoomerang)
		weights = append(weights, 2.0)
	}

	// Approach if not in melee range - always an option
	if dist > combat.ApproachMinRange {
		options = append(options, AttackApproach)
		weights = append(weights, 3.0)
	}

	if len(options) == 0 {
		return AttackApproach
	}

	// Weighted random selection
	totalWeight := 0.0
	for _, w := range weights {
		totalWeight += w
	}

	roll := rng.Float64() * totalWeight
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if roll < cumulative {
			return options[i]
		}
	}

	return options[len(options)-1]
}

// ThreatType identifies what kind of attack is incoming
type ThreatType int

const (
	ThreatNone      ThreatType = iota
	ThreatBoomerang            // Incoming boomerang projectile
	ThreatMelee                // Player in attack animation nearby
)

// ThreatInfo describes an incoming attack
type ThreatInfo struct {
	Type      ThreatType
	X, Y      float64 // Threat position
	VelX      float64 // Horizontal velocity (for boomerangs)
	VelY      float64 // Vertical velocity
	TimeToHit int     // Frames until impact (estimate)
}

// Pre-allocated threat info to avoid allocations in hot path
var cachedThreatInfo ThreatInfo

// detectIncomingThreats scans for boomerangs and attacking players
// Returns pointer to cached ThreatInfo (valid until next call) or nil if no threat
func detectIncomingThreats(e *ecs.ECS, botX, botY float64, botPlayerIndex int) *ThreatInfo {
	foundThreat := false
	closestTime := 999

	// Check for incoming boomerangs
	components.Boomerang.Each(e.World, func(entry *donburi.Entry) {
		b := components.Boomerang.Get(entry)

		// Skip own boomerang
		if b.OwnerIndex == botPlayerIndex {
			return
		}

		obj := components.Object.Get(entry)
		physics := components.Physics.Get(entry)

		// Is boomerang heading toward us?
		dx := botX - obj.X
		dy := botY - obj.Y
		dist := math.Sqrt(dx*dx + dy*dy)

		// Check if boomerang is moving toward bot
		dotProduct := dx*physics.SpeedX + dy*physics.SpeedY
		if dotProduct <= 0 {
			return // Moving away from us
		}

		// Estimate time to impact
		speed := math.Sqrt(physics.SpeedX*physics.SpeedX + physics.SpeedY*physics.SpeedY)
		if speed < 0.1 {
			return
		}
		timeToHit := int(dist / speed)

		// Only react if boomerang is close and heading our way
		if dist < 200 && timeToHit < closestTime && timeToHit < 60 {
			closestTime = timeToHit
			foundThreat = true
			cachedThreatInfo.Type = ThreatBoomerang
			cachedThreatInfo.X = obj.X
			cachedThreatInfo.Y = obj.Y
			cachedThreatInfo.VelX = physics.SpeedX
			cachedThreatInfo.VelY = physics.SpeedY
			cachedThreatInfo.TimeToHit = timeToHit
		}
	})

	// Check for nearby attacking players
	tags.Player.Each(e.World, func(entry *donburi.Entry) {
		player := components.Player.Get(entry)

		// Skip self
		if player.PlayerIndex == botPlayerIndex {
			return
		}

		// Skip if player not in attack state
		state := components.State.Get(entry)
		if state.CurrentState != cfg.Punch01 && state.CurrentState != cfg.Kick01 &&
			state.CurrentState != cfg.StateAttackingPunch && state.CurrentState != cfg.StateAttackingKick {
			return
		}

		obj := components.Object.Get(entry)
		dx := botX - obj.X
		dy := botY - obj.Y
		dist := math.Sqrt(dx*dx + dy*dy)

		// React to nearby attackers
		if dist < 80 {
			timeToHit := 10 // Melee is fast
			if timeToHit < closestTime {
				closestTime = timeToHit
				foundThreat = true
				cachedThreatInfo.Type = ThreatMelee
				cachedThreatInfo.X = obj.X
				cachedThreatInfo.Y = obj.Y
				cachedThreatInfo.VelX = float64(player.Direction.X) * 5
				cachedThreatInfo.VelY = 0
				cachedThreatInfo.TimeToHit = timeToHit
			}
		}
	})

	if foundThreat {
		return &cachedThreatInfo
	}
	return nil
}

// generateDefensiveInputs reacts to incoming threats
func generateDefensiveInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, threat *ThreatInfo, botX, botY float64, physics *components.PhysicsData) bool {
	if threat == nil {
		return false // No threat, don't override other inputs
	}

	// Only react to boomerangs - melee is handled by normal combat
	// This reduces excessive defensive jumping
	if threat.Type != ThreatBoomerang {
		return false
	}

	// Only react if threat is imminent based on difficulty
	reactionFrames := bot.ReactionDelay + 5 // Give a bit more buffer
	if threat.TimeToHit > reactionFrames {
		return false // Too far away to react yet
	}

	// Determine best evasion based on threat trajectory
	threatComingFromLeft := threat.VelX > 0
	threatComingHigh := threat.Y < botY-30 // Increased threshold
	threatComingLow := threat.Y > botY+30  // Increased threshold

	// Boomerangs: duck if coming high, jump if coming low
	if threatComingHigh {
		// DUCK - boomerang will pass overhead
		input.CurrentInput[cfg.ActionCrouch] = true
		return true
	} else if threatComingLow {
		// JUMP - boomerang will pass underneath (only if clearly low)
		if physics.OnGround != nil && bot.JumpCooldown <= 0 {
			input.CurrentInput[cfg.ActionJump] = true
			bot.JumpCooldown = 40 // Longer cooldown
			return true
		}
	}

	// For mid-height boomerangs, just try to move away instead of jumping
	if threatComingFromLeft {
		input.CurrentInput[cfg.ActionMoveRight] = true
	} else {
		input.CurrentInput[cfg.ActionMoveLeft] = true
	}
	return true
}

// getOrCreateNavGrid returns cached nav grid or creates a new one
func getOrCreateNavGrid(e *ecs.ECS, space *resolv.Space) *NavGrid {
	if space == nil {
		return nil
	}

	// Get level data to check if we need to rebuild
	levelEntry, ok := components.Level.First(e.World)
	if !ok {
		return nil
	}
	levelData := components.Level.Get(levelEntry)
	if levelData.CurrentLevel == nil {
		return nil
	}
	currentLevelID := levelData.CurrentLevel.Name

	// Return cached grid if still valid
	if cachedNavGrid != nil && navGridLevelID == currentLevelID {
		return cachedNavGrid
	}

	// Build new nav grid
	// Cell size of 32 pixels works well for character-sized navigation
	cellSize := 32.0
	levelWidth := levelData.CurrentLevel.Width
	levelHeight := levelData.CurrentLevel.Height

	cachedNavGrid = CreateNavGrid(space, levelWidth, levelHeight, cellSize)
	navGridLevelID = currentLevelID

	return cachedNavGrid
}
