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

var rng = rand.New(rand.NewSource(42)) // Fixed seed for deterministic replay

// Nav grid cache (created once per level)
var cachedNavGrid *NavGrid
var navGridLevelID string

// UpdateBots generates input for bot-controlled players.
// Must run BEFORE UpdateMultiPlayerInput.
func UpdateBots(e *ecs.ECS) {
	if !IsMatchPlaying(e) {
		return
	}

	var space *resolv.Space
	if spaceEntry, ok := components.Space.First(e.World); ok {
		space = components.Space.Get(spaceEntry)
	}

	navGrid := getOrCreateNavGrid(e, space)

	var match *components.MatchData
	if matchEntry, ok := components.Match.First(e.World); ok {
		match = components.Match.Get(matchEntry)
	}

	var playerPositions []playerInfo
	tags.Player.Each(e.World, func(entry *donburi.Entry) {
		obj := components.Object.Get(entry)
		player := components.Player.Get(entry)
		health := components.Health.Get(entry)

		team := -1
		if match != nil {
			team = match.GetPlayerScore(player.PlayerIndex).Team
		}

		playerPositions = append(playerPositions, playerInfo{
			entry:       entry,
			playerIndex: player.PlayerIndex,
			x:           obj.X + obj.W/2,
			y:           obj.Y + obj.H/2,
			health:      health.Current,
			maxHealth:   health.Max,
			isBot:       entry.HasComponent(components.Bot),
			team:        team,
		})
	})

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
	team        int
}

func updateBotAI(e *ecs.ECS, botEntry *donburi.Entry, players []playerInfo, space *resolv.Space, navGrid *NavGrid) {
	bot := components.Bot.Get(botEntry)
	input := components.PlayerInput.Get(botEntry)
	player := components.Player.Get(botEntry)
	obj := components.Object.Get(botEntry)
	health := components.Health.Get(botEntry)
	physics := components.Physics.Get(botEntry)

	if bot.DecisionTimer > 0 {
		bot.DecisionTimer--
	}
	if bot.AttackCooldown > 0 {
		bot.AttackCooldown--
	}
	if bot.JumpCooldown > 0 {
		bot.JumpCooldown--
	}

	input.PreviousInput = input.CurrentInput
	input.CurrentInput = [cfg.ActionCount]bool{}

	botX := obj.X + obj.W/2
	botY := obj.Y + obj.H/2

	botTeam := -1
	for i := range players {
		if players[i].playerIndex == player.PlayerIndex {
			botTeam = players[i].team
			break
		}
	}

	target := findNearestTarget(bot, player.PlayerIndex, botTeam, botX, botY, players)
	teammates := findNearbyTeammates(player.PlayerIndex, botTeam, botX, botY, players)

	if target != nil {
		bot.TargetPlayerIndex = target.playerIndex
		bot.TargetX = target.x
		bot.TargetY = target.y
		bot.DistanceToTarget = distance(botX, botY, target.x, target.y)
	} else {
		bot.TargetPlayerIndex = -1
		bot.DistanceToTarget = 9999
	}

	// Threat detection takes priority
	threat := detectIncomingThreats(e, botX, botY, player.PlayerIndex, botTeam, players)
	if generateDefensiveInputs(bot, input, player, threat, botX, botY, physics) {
		return
	}

	healthPercent := float64(health.Current) / float64(health.Max)

	if bot.DecisionTimer <= 0 {
		updateBotState(bot, target, healthPercent, physics)
		bot.DecisionTimer = bot.ReactionDelay / 3
	}

	generateBotInputs(bot, input, player, obj, physics, target, teammates, botX, botY, space, navGrid)
}

func findNearestTarget(bot *components.BotData, myIndex int, myTeam int, myX, myY float64, players []playerInfo) *playerInfo {
	var nearest *playerInfo
	nearestDist := math.MaxFloat64

	for i := range players {
		p := &players[i]
		if p.playerIndex == myIndex {
			continue
		}
		if p.health <= 0 {
			continue
		}
		if myTeam != -1 && p.team == myTeam {
			continue
		}

		dist := distance(myX, myY, p.x, p.y)
		if dist < nearestDist {
			nearestDist = dist
			nearest = p
		}
	}

	return nearest
}

// Pre-allocated slice for teammate detection (avoids allocation per frame)
var cachedTeammates = make([]*playerInfo, 0, 4)

// findNearbyTeammates returns teammates within detection range (for collision avoidance).
// Returns pointers into the players slice - valid only for current frame.
func findNearbyTeammates(myIndex int, myTeam int, myX, myY float64, players []playerInfo) []*playerInfo {
	cachedTeammates = cachedTeammates[:0]

	if myTeam == -1 {
		return cachedTeammates
	}

	for i := range players {
		p := &players[i]
		if p.playerIndex == myIndex || p.health <= 0 || p.team != myTeam {
			continue
		}
		if distance(myX, myY, p.x, p.y) < cfg.Pathfinding.TeammateDetectRange {
			cachedTeammates = append(cachedTeammates, p)
		}
	}

	return cachedTeammates
}

// isTeammateBlocking checks if a teammate is blocking movement in a direction
func isTeammateBlocking(botX, botY float64, movingRight bool, teammates []*playerInfo) *playerInfo {
	for _, t := range teammates {
		dx := t.x - botX
		dy := t.y - botY

		if math.Abs(dy) > cfg.Pathfinding.TeammateVerticalTolerance {
			continue
		}

		blockDist := cfg.Pathfinding.TeammateBlockingDist
		if movingRight && dx > 0 && dx < blockDist {
			return t
		}
		if !movingRight && dx < 0 && dx > -blockDist {
			return t
		}
	}
	return nil
}

func updateBotState(bot *components.BotData, target *playerInfo, healthPercent float64, physics *components.PhysicsData) {
	if healthPercent < bot.RetreatThreshold && target != nil {
		bot.AIState = components.BotStateRetreat
		return
	}

	if target == nil {
		bot.AIState = components.BotStateIdle
		return
	}

	if bot.DistanceToTarget < bot.AttackRange {
		bot.AIState = components.BotStateAttack
		return
	}

	bot.AIState = components.BotStateChase
}

func generateBotInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, obj *components.ObjectData, physics *components.PhysicsData, target *playerInfo, teammates []*playerInfo, botX, botY float64, space *resolv.Space, navGrid *NavGrid) {
	switch bot.AIState {
	case components.BotStateIdle:
		generateIdleInputs(bot, input, player, obj, physics, space, teammates, botX, botY)
	case components.BotStateChase:
		generateChaseInputs(bot, input, player, target, botX, botY, obj, physics, space, navGrid, teammates)
	case components.BotStateAttack:
		generateAttackInputs(bot, input, player, target, botX, botY, physics, space, teammates)
	case components.BotStateRetreat:
		generateRetreatInputs(bot, input, player, target, botX, botY, obj, physics, space, teammates)
	}
}

func generateIdleInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, obj *components.ObjectData, physics *components.PhysicsData, space *resolv.Space, teammates []*playerInfo, botX, botY float64) {
	// Patrol back and forth looking for enemies
	bot.IdleTimer++

	// Change direction every ~3 seconds or when hitting a wall/gap
	if bot.IdleTimer > 180 {
		bot.IdleTimer = 0
		bot.PatrolDirection = -bot.PatrolDirection
	}

	// Initialize patrol direction if not set
	if bot.PatrolDirection == 0 {
		if rng.Float32() < 0.5 {
			bot.PatrolDirection = 1
		} else {
			bot.PatrolDirection = -1
		}
	}

	movingRight := bot.PatrolDirection > 0

	// Check for teammate blocking path - turn around or jump over
	if blocker := isTeammateBlocking(botX, botY, movingRight, teammates); blocker != nil {
		// Try to jump over if on ground
		if physics.OnGround != nil && bot.JumpCooldown <= 0 {
			input.CurrentInput[cfg.ActionJump] = true
			bot.JumpCooldown = 30
		} else {
			// Turn around
			bot.PatrolDirection = -bot.PatrolDirection
			bot.IdleTimer = 0
		}
		return
	}

	// Check for gaps and walls
	if physics.OnGround != nil {
		if gapDetected, gapWidth := detectGapAhead(space, obj, movingRight); gapDetected {
			if gapWidth < cfg.Pathfinding.MaxJumpDistance && bot.JumpCooldown <= 0 {
				input.CurrentInput[cfg.ActionJump] = true
				bot.JumpCooldown = 20
			} else {
				// Gap too wide, turn around
				bot.PatrolDirection = -bot.PatrolDirection
				bot.IdleTimer = 0
			}
		}
	}

	// Move in patrol direction
	if bot.PatrolDirection > 0 {
		input.CurrentInput[cfg.ActionMoveRight] = true
		player.Direction.X = cfg.DirectionRight
	} else {
		input.CurrentInput[cfg.ActionMoveLeft] = true
		player.Direction.X = cfg.DirectionLeft
	}
}

func generateChaseInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *playerInfo, botX, botY float64, obj *components.ObjectData, physics *components.PhysicsData, space *resolv.Space, navGrid *NavGrid, teammates []*playerInfo) {
	if target == nil {
		return
	}

	dx := target.x - botX
	dy := target.y - botY
	movingRight := dx > 0

	// Set facing direction
	player.Direction.X = cfg.DirectionLeft
	if movingRight {
		player.Direction.X = cfg.DirectionRight
	}

	// Handle teammate blocking - jump over or wait
	if blocker := isTeammateBlocking(botX, botY, movingRight, teammates); blocker != nil {
		handleTeammateBlocking(bot, input, physics, movingRight, blocker, botY)
		return
	}

	// Move toward target
	if dx > 10 {
		input.CurrentInput[cfg.ActionMoveRight] = true
	} else if dx < -10 {
		input.CurrentInput[cfg.ActionMoveLeft] = true
	}

	// Jump over gaps
	if physics.OnGround != nil && bot.JumpCooldown <= 0 {
		if gapDetected, gapWidth := detectGapAhead(space, obj, movingRight); gapDetected && gapWidth < cfg.Pathfinding.MaxJumpDistance {
			input.CurrentInput[cfg.ActionJump] = true
			bot.JumpCooldown = 20
		}
	}

	// Jump if target is above
	if dy < -60 && physics.OnGround != nil && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 45
	}

	// Wall jump - longer cooldown to prevent rapid bouncing in corridors
	if physics.WallSliding != nil && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 45
	}
}

// handleTeammateBlocking handles movement when a teammate is in the way
func handleTeammateBlocking(bot *components.BotData, input *components.PlayerInputData, physics *components.PhysicsData, movingRight bool, blocker *playerInfo, botY float64) {
	onGround := physics.OnGround != nil
	canJump := bot.JumpCooldown <= 0

	// Jump over teammate if possible
	if onGround && canJump {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 25
	}

	// Continue moving if in air or teammate is below us
	inAir := !onGround
	teammateBelow := blocker.y-botY > 5

	if inAir || teammateBelow {
		if movingRight {
			input.CurrentInput[cfg.ActionMoveRight] = true
		} else {
			input.CurrentInput[cfg.ActionMoveLeft] = true
		}
	}
	// Otherwise wait briefly for teammate to move
}

func generateAttackInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *playerInfo, botX, botY float64, physics *components.PhysicsData, space *resolv.Space, teammates []*playerInfo) {
	if target == nil {
		return
	}

	dx := target.x - botX
	dy := target.y - botY
	movingRight := dx > 0

	if dx > 0 {
		player.Direction.X = cfg.DirectionRight
	} else {
		player.Direction.X = cfg.DirectionLeft
	}

	dist := bot.DistanceToTarget

	// Move toward target if not in melee range
	if dist > 40 {
		// Check for teammate blocking
		if blocker := isTeammateBlocking(botX, botY, movingRight, teammates); blocker != nil {
			// Jump over teammate
			if physics.OnGround != nil && bot.JumpCooldown <= 0 {
				input.CurrentInput[cfg.ActionJump] = true
				bot.JumpCooldown = 25
			}
		}
		if dx > 0 {
			input.CurrentInput[cfg.ActionMoveRight] = true
		} else {
			input.CurrentInput[cfg.ActionMoveLeft] = true
		}
	}

	if bot.AttackCooldown > 0 {
		return
	}

	hasLOS := hasLineOfSight(space, botX, botY, target.x, target.y)
	onGround := physics.OnGround != nil
	attack := chooseAttack(bot, dist, dy, hasLOS, onGround)

	switch attack {
	case AttackPunch:
		input.CurrentInput[cfg.ActionAttack] = true
		bot.AttackCooldown = 20 + bot.ReactionDelay/2

	case AttackJumpKick:
		if bot.JumpCooldown <= 0 {
			input.CurrentInput[cfg.ActionJump] = true
			input.CurrentInput[cfg.ActionAttack] = true
			bot.JumpCooldown = 50
			bot.AttackCooldown = 25
		}

	case AttackBoomerang:
		if dy < -30 {
			input.CurrentInput[cfg.ActionMoveUp] = true
		}
		input.CurrentInput[cfg.ActionBoomerang] = true
		bot.AttackCooldown = 90
	}
}

func generateRetreatInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *playerInfo, botX, botY float64, obj *components.ObjectData, physics *components.PhysicsData, space *resolv.Space, teammates []*playerInfo) {
	if target == nil {
		return
	}

	dx := target.x - botX
	dy := target.y - botY
	dist := bot.DistanceToTarget

	// Face target
	if dx > 0 {
		player.Direction.X = cfg.DirectionRight
	} else {
		player.Direction.X = cfg.DirectionLeft
	}

	// Update evasion timer for strafing pattern
	bot.IdleTimer++

	// Evasive movement pattern - strafe back and forth while fighting
	strafePhase := (bot.IdleTimer / 30) % 4 // Change direction every 0.5 seconds

	// Determine intended movement direction
	var intendedMoveRight, intendedMoveLeft bool

	if dist < 60 {
		// Too close - back away with random dodges
		if dx > 0 {
			intendedMoveLeft = true
		} else {
			intendedMoveRight = true
		}
		// Panic jump when very close
		if dist < 35 && physics.OnGround != nil && bot.JumpCooldown <= 0 {
			input.CurrentInput[cfg.ActionJump] = true
			bot.JumpCooldown = 40
		}
	} else if dist < 150 {
		// Mid-range - strafe unpredictably while attacking
		switch strafePhase {
		case 0:
			intendedMoveLeft = true
		case 1:
			// Brief pause
		case 2:
			intendedMoveRight = true
		case 3:
			// Brief pause or approach
			if dist > 100 {
				if dx > 0 {
					intendedMoveRight = true
				} else {
					intendedMoveLeft = true
				}
			}
		}
	} else {
		// Far away - chase the target
		if dx > 10 {
			intendedMoveRight = true
		} else if dx < -10 {
			intendedMoveLeft = true
		}
	}

	// Check for teammate blocking intended movement
	if intendedMoveRight || intendedMoveLeft {
		movingRight := intendedMoveRight
		if blocker := isTeammateBlocking(botX, botY, movingRight, teammates); blocker != nil {
			// Jump over teammate
			if physics.OnGround != nil && bot.JumpCooldown <= 0 {
				input.CurrentInput[cfg.ActionJump] = true
				bot.JumpCooldown = 25
			}
		}
		if intendedMoveRight {
			input.CurrentInput[cfg.ActionMoveRight] = true
		}
		if intendedMoveLeft {
			input.CurrentInput[cfg.ActionMoveLeft] = true
		}
	}

	// Determine current movement direction for gap detection
	movingRight := input.CurrentInput[cfg.ActionMoveRight]
	movingLeft := input.CurrentInput[cfg.ActionMoveLeft]

	// Jump over gaps
	if physics.OnGround != nil && bot.JumpCooldown <= 0 && (movingRight || movingLeft) {
		if gapDetected, gapWidth := detectGapAhead(space, obj, movingRight); gapDetected && gapWidth < cfg.Pathfinding.MaxJumpDistance {
			input.CurrentInput[cfg.ActionJump] = true
			bot.JumpCooldown = 20
		}
	}

	// Jump to reach target on higher platforms
	if dy < -60 && physics.OnGround != nil && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 45
	}

	// Evasive jumps - occasionally jump to be unpredictable
	if physics.OnGround != nil && bot.JumpCooldown <= 0 && rng.Float32() < 0.02 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 50
	}

	// Wall jump to escape - longer cooldown to prevent rapid bouncing
	if physics.WallSliding != nil && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 45
	}

	// Aggressive counter-attacks
	if bot.AttackCooldown <= 0 {
		if dist > 80 && dist < 250 {
			// Boomerang at range
			input.CurrentInput[cfg.ActionBoomerang] = true
			bot.AttackCooldown = 60
		} else if dist < 80 && dist > 50 {
			// Jump kick to close distance aggressively
			if physics.OnGround != nil && rng.Float32() < 0.4 {
				input.CurrentInput[cfg.ActionJump] = true
				input.CurrentInput[cfg.ActionAttack] = true
				bot.AttackCooldown = 45
				bot.JumpCooldown = 30
			} else {
				input.CurrentInput[cfg.ActionAttack] = true
				bot.AttackCooldown = 25
			}
		} else if dist <= 50 {
			// Melee range - punch
			input.CurrentInput[cfg.ActionAttack] = true
			bot.AttackCooldown = 20
		}
	}
}

func distance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// detectGapAhead checks for gaps ahead and returns (detected, width).
func detectGapAhead(space *resolv.Space, obj *components.ObjectData, movingRight bool) (bool, float64) {
	if space == nil {
		return false, 0
	}

	checkDist := cfg.Pathfinding.GapCheckDist
	checkDepth := cfg.Pathfinding.GapCheckDepth
	maxScanDist := cfg.Pathfinding.MaxJumpDistance
	scanStep := cfg.Pathfinding.LOSStepSize

	startY := obj.Y + obj.H

	var startX float64
	if movingRight {
		startX = obj.X + obj.W + checkDist
	} else {
		startX = obj.X - checkDist
	}

	// Check for ground below
	for _, checkObj := range space.Objects() {
		if !checkObj.HasTags(tags.ResolvSolid) {
			continue
		}
		if startX >= checkObj.X && startX <= checkObj.X+checkObj.W &&
			checkObj.Y >= startY && checkObj.Y <= startY+checkDepth {
			return false, 0
		}
	}

	// Measure gap width
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
			if scanX >= checkObj.X && scanX <= checkObj.X+checkObj.W &&
				checkObj.Y >= startY && checkObj.Y <= startY+checkDepth*2 {
				return true, dist - checkDist
			}
		}
	}

	return true, maxScanDist
}

// hasLineOfSight checks for clear path using manual AABB intersection.
func hasLineOfSight(space *resolv.Space, x1, y1, x2, y2 float64) bool {
	if space == nil {
		return true
	}

	dx := x2 - x1
	dy := y2 - y1
	dist := math.Sqrt(dx*dx + dy*dy)

	if dist == 0 {
		return true
	}

	dx /= dist
	dy /= dist

	stepSize := cfg.Pathfinding.LOSStepSize
	checkSize := cfg.Pathfinding.LOSCheckSize

	for d := stepSize; d < dist-stepSize; d += stepSize {
		checkX := x1 + dx*d
		checkY := y1 + dy*d

		for _, obj := range space.Objects() {
			if !obj.HasTags(tags.ResolvSolid) {
				continue
			}
			if checkX+checkSize > obj.X && checkX-checkSize < obj.X+obj.W &&
				checkY+checkSize > obj.Y && checkY-checkSize < obj.Y+obj.H {
				return false
			}
		}
	}

	return true
}

type AttackChoice int

const (
	AttackPunch AttackChoice = iota
	AttackJumpKick
	AttackBoomerang
	AttackApproach
)

// Pre-allocated buffers to avoid allocations
var (
	attackOptions = make([]AttackChoice, 0, 4)
	attackWeights = make([]float64, 0, 4)
)

func chooseAttack(bot *components.BotData, dist float64, dy float64, hasLOS bool, onGround bool) AttackChoice {
	options := attackOptions[:0]
	weights := attackWeights[:0]
	combat := &cfg.BotCombat

	if dist < combat.PunchRange {
		options = append(options, AttackPunch)
		weights = append(weights, 5.0)
	}

	if dist < combat.JumpKickMaxRange && dist > combat.JumpKickMinRange && onGround {
		options = append(options, AttackJumpKick)
		weights = append(weights, 0.5)
	}

	if dist > combat.BoomerangMinRange && dist < combat.BoomerangMaxRange && hasLOS {
		options = append(options, AttackBoomerang)
		weights = append(weights, 2.0)
	}

	if dist > combat.ApproachMinRange {
		options = append(options, AttackApproach)
		weights = append(weights, 3.0)
	}

	if len(options) == 0 {
		return AttackApproach
	}

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

type ThreatType int

const (
	ThreatNone ThreatType = iota
	ThreatBoomerang
	ThreatMelee
)

type ThreatInfo struct {
	Type      ThreatType
	X, Y      float64
	VelX      float64
	VelY      float64
	TimeToHit int
}

var cachedThreatInfo ThreatInfo

func detectIncomingThreats(e *ecs.ECS, botX, botY float64, botPlayerIndex int, botTeam int, players []playerInfo) *ThreatInfo {
	foundThreat := false
	closestTime := 999

	getPlayerTeam := func(playerIndex int) int {
		for i := range players {
			if players[i].playerIndex == playerIndex {
				return players[i].team
			}
		}
		return -1
	}

	components.Boomerang.Each(e.World, func(entry *donburi.Entry) {
		b := components.Boomerang.Get(entry)

		if b.OwnerIndex == botPlayerIndex {
			return
		}

		if botTeam != -1 && getPlayerTeam(b.OwnerIndex) == botTeam {
			return
		}

		obj := components.Object.Get(entry)
		physics := components.Physics.Get(entry)

		dx := botX - obj.X
		dy := botY - obj.Y
		dist := math.Sqrt(dx*dx + dy*dy)

		dotProduct := dx*physics.SpeedX + dy*physics.SpeedY
		if dotProduct <= 0 {
			return
		}

		speed := math.Sqrt(physics.SpeedX*physics.SpeedX + physics.SpeedY*physics.SpeedY)
		if speed < 0.1 {
			return
		}
		timeToHit := int(dist / speed)

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

	tags.Player.Each(e.World, func(entry *donburi.Entry) {
		playerData := components.Player.Get(entry)

		if playerData.PlayerIndex == botPlayerIndex {
			return
		}

		if botTeam != -1 && getPlayerTeam(playerData.PlayerIndex) == botTeam {
			return
		}

		state := components.State.Get(entry)
		if state.CurrentState != cfg.Punch01 && state.CurrentState != cfg.Kick01 &&
			state.CurrentState != cfg.StateAttackingPunch && state.CurrentState != cfg.StateAttackingKick {
			return
		}

		obj := components.Object.Get(entry)
		dx := botX - obj.X
		dy := botY - obj.Y
		dist := math.Sqrt(dx*dx + dy*dy)

		if dist < 80 {
			timeToHit := 10
			if timeToHit < closestTime {
				closestTime = timeToHit
				foundThreat = true
				cachedThreatInfo.Type = ThreatMelee
				cachedThreatInfo.X = obj.X
				cachedThreatInfo.Y = obj.Y
				cachedThreatInfo.VelX = float64(playerData.Direction.X) * 5
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

func generateDefensiveInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, threat *ThreatInfo, botX, botY float64, physics *components.PhysicsData) bool {
	if threat == nil {
		return false
	}

	// Only react to boomerangs
	if threat.Type != ThreatBoomerang {
		return false
	}

	reactionFrames := bot.ReactionDelay + 5
	if threat.TimeToHit > reactionFrames {
		return false
	}

	threatComingFromLeft := threat.VelX > 0
	threatComingHigh := threat.Y < botY-30
	threatComingLow := threat.Y > botY+30

	if threatComingHigh {
		input.CurrentInput[cfg.ActionCrouch] = true
		return true
	} else if threatComingLow && physics.OnGround != nil && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 40
		return true
	}

	if threatComingFromLeft {
		input.CurrentInput[cfg.ActionMoveRight] = true
	} else {
		input.CurrentInput[cfg.ActionMoveLeft] = true
	}
	return true
}

func getOrCreateNavGrid(e *ecs.ECS, space *resolv.Space) *NavGrid {
	if space == nil {
		return nil
	}

	levelEntry, ok := components.Level.First(e.World)
	if !ok {
		return nil
	}
	levelData := components.Level.Get(levelEntry)
	if levelData.CurrentLevel == nil {
		return nil
	}
	currentLevelID := levelData.CurrentLevel.Name

	if cachedNavGrid != nil && navGridLevelID == currentLevelID {
		return cachedNavGrid
	}

	cachedNavGrid = CreateNavGrid(space, levelData.CurrentLevel.Width, levelData.CurrentLevel.Height, 32.0)
	navGridLevelID = currentLevelID

	return cachedNavGrid
}
