package botai

import (
	"math"
	"math/rand"

	"github.com/solarlune/resolv"

	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/mathutil"
	"github.com/automoto/doomerang-mp/shared/pathfinding"
	"github.com/automoto/doomerang-mp/tags"
)

type PlayerInfo struct {
	Index        int
	X, Y         float64
	W, H         float64
	Health       int
	MaxHealth    int
	IsBot        bool
	Team         int
	CurrentState cfg.StateID
}

type PhysicsInfo struct {
	OnGround    bool
	WallSliding bool
	SpeedX      float64
	SpeedY      float64
}

type BoomerangInfo struct {
	OwnerIndex int
	X, Y       float64
	SpeedX     float64
	SpeedY     float64
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

type AttackChoice int

const (
	AttackPunch AttackChoice = iota
	AttackJumpKick
	AttackBoomerang
	AttackApproach
)

func UpdateBotAI(
	rng *rand.Rand,
	bot *components.BotData,
	input *components.PlayerInputData,
	player *components.PlayerData,
	objX, objY, objW, objH float64,
	healthCurrent, healthMax int,
	physics PhysicsInfo,
	players []PlayerInfo,
	boomerangs []BoomerangInfo,
	space *resolv.Space,
	navGrid *pathfinding.NavGrid,
) {
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

	botX := objX + objW/2
	botY := objY + objH/2

	botTeam := -1
	for i := range players {
		if players[i].Index == player.PlayerIndex {
			botTeam = players[i].Team
			break
		}
	}

	target := FindNearestTarget(bot, player.PlayerIndex, botTeam, botX, botY, players)
	teammates := FindNearbyTeammates(player.PlayerIndex, botTeam, botX, botY, players)

	if target != nil {
		bot.TargetPlayerIndex = target.Index
		bot.TargetX = target.X
		bot.TargetY = target.Y
		bot.DistanceToTarget = mathutil.Distance(botX, botY, target.X, target.Y)
	} else {
		bot.TargetPlayerIndex = -1
		bot.DistanceToTarget = 9999
	}

	// Threat detection takes priority
	threat := DetectIncomingThreats(botX, botY, player.PlayerIndex, botTeam, players, boomerangs)
	if GenerateDefensiveInputs(bot, input, player, threat, botX, botY, physics) {
		return
	}

	healthPercent := float64(healthCurrent) / float64(healthMax)

	if bot.DecisionTimer <= 0 {
		UpdateBotState(bot, target, healthPercent)
		bot.DecisionTimer = bot.ReactionDelay / 3
	}

	GenerateBotInputs(rng, bot, input, player, objX, objY, objW, objH, physics, target, teammates, botX, botY, space, navGrid)
}

func FindNearestTarget(bot *components.BotData, myIndex int, myTeam int, myX, myY float64, players []PlayerInfo) *PlayerInfo {
	var nearest *PlayerInfo
	nearestDist := math.MaxFloat64

	for i := range players {
		p := &players[i]
		if p.Index == myIndex {
			continue
		}
		if p.Health <= 0 {
			continue
		}
		if myTeam != -1 && p.Team == myTeam {
			continue
		}

		dist := mathutil.Distance(myX, myY, p.X, p.Y)
		if dist < nearestDist {
			nearestDist = dist
			nearest = p
		}
	}

	return nearest
}

func FindNearbyTeammates(myIndex int, myTeam int, myX, myY float64, players []PlayerInfo) []PlayerInfo {
	var teammates []PlayerInfo

	if myTeam == -1 {
		return teammates
	}

	for i := range players {
		p := &players[i]
		if p.Index == myIndex || p.Health <= 0 || p.Team != myTeam {
			continue
		}
		if mathutil.Distance(myX, myY, p.X, p.Y) < cfg.Pathfinding.TeammateDetectRange {
			teammates = append(teammates, *p)
		}
	}

	return teammates
}

func IsTeammateBlocking(botX, botY float64, movingRight bool, teammates []PlayerInfo) *PlayerInfo {
	for i := range teammates {
		t := &teammates[i]
		dx := t.X - botX
		dy := t.Y - botY

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

func UpdateBotState(bot *components.BotData, target *PlayerInfo, healthPercent float64) {
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

func GenerateBotInputs(rng *rand.Rand, bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, objX, objY, objW, objH float64, physics PhysicsInfo, target *PlayerInfo, teammates []PlayerInfo, botX, botY float64, space *resolv.Space, navGrid *pathfinding.NavGrid) {
	switch bot.AIState {
	case components.BotStateIdle:
		GenerateIdleInputs(rng, bot, input, player, objX, objY, objW, objH, physics, space, teammates, botX, botY)
	case components.BotStateChase:
		GenerateChaseInputs(bot, input, player, target, botX, botY, objX, objY, objW, objH, physics, space, navGrid, teammates)
	case components.BotStateAttack:
		GenerateAttackInputs(rng, bot, input, player, target, botX, botY, physics, space, teammates)
	case components.BotStateRetreat:
		GenerateRetreatInputs(rng, bot, input, player, target, botX, botY, objX, objY, objW, objH, physics, space, teammates)
	}
}

func GenerateIdleInputs(rng *rand.Rand, bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, objX, objY, objW, objH float64, physics PhysicsInfo, space *resolv.Space, teammates []PlayerInfo, botX, botY float64) {
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
	if blocker := IsTeammateBlocking(botX, botY, movingRight, teammates); blocker != nil {
		// Try to jump over if on ground
		if physics.OnGround && bot.JumpCooldown <= 0 {
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
	if physics.OnGround {
		if gapDetected, gapWidth := DetectGapAhead(space, objX, objY, objW, objH, movingRight); gapDetected {
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

func GenerateChaseInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *PlayerInfo, botX, botY float64, objX, objY, objW, objH float64, physics PhysicsInfo, space *resolv.Space, navGrid *pathfinding.NavGrid, teammates []PlayerInfo) {
	if target == nil {
		return
	}

	dx := target.X - botX
	dy := target.Y - botY
	movingRight := dx > 0

	// Set facing direction
	player.Direction.X = cfg.DirectionLeft
	if movingRight {
		player.Direction.X = cfg.DirectionRight
	}

	// Handle teammate blocking - jump over or wait
	if blocker := IsTeammateBlocking(botX, botY, movingRight, teammates); blocker != nil {
		HandleTeammateBlocking(bot, input, physics, movingRight, blocker, botY)
		return
	}

	// Move toward target
	if dx > 10 {
		input.CurrentInput[cfg.ActionMoveRight] = true
	} else if dx < -10 {
		input.CurrentInput[cfg.ActionMoveLeft] = true
	}

	// Jump over gaps
	if physics.OnGround && bot.JumpCooldown <= 0 {
		if gapDetected, gapWidth := DetectGapAhead(space, objX, objY, objW, objH, movingRight); gapDetected && gapWidth < cfg.Pathfinding.MaxJumpDistance {
			input.CurrentInput[cfg.ActionJump] = true
			bot.JumpCooldown = 20
		}
	}

	// Jump if target is above
	if dy < -60 && physics.OnGround && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 45
	}

	// Wall jump - longer cooldown to prevent rapid bouncing in corridors
	if physics.WallSliding && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 45
	}
}

func HandleTeammateBlocking(bot *components.BotData, input *components.PlayerInputData, physics PhysicsInfo, movingRight bool, blocker *PlayerInfo, botY float64) {
	onGround := physics.OnGround
	canJump := bot.JumpCooldown <= 0

	// Jump over teammate if possible
	if onGround && canJump {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 25
	}

	// Continue moving if in air or teammate is below us
	inAir := !onGround
	teammateBelow := blocker.Y-botY > 5

	if inAir || teammateBelow {
		if movingRight {
			input.CurrentInput[cfg.ActionMoveRight] = true
		} else {
			input.CurrentInput[cfg.ActionMoveLeft] = true
		}
	}
	// Otherwise wait briefly for teammate to move
}

func GenerateAttackInputs(rng *rand.Rand, bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *PlayerInfo, botX, botY float64, physics PhysicsInfo, space *resolv.Space, teammates []PlayerInfo) {
	if target == nil {
		return
	}

	dx := target.X - botX
	dy := target.Y - botY
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
		if blocker := IsTeammateBlocking(botX, botY, movingRight, teammates); blocker != nil {
			// Jump over teammate
			if physics.OnGround && bot.JumpCooldown <= 0 {
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

	hasLOS := HasLineOfSight(space, botX, botY, target.X, target.Y)
	onGround := physics.OnGround
	attack := ChooseAttack(rng, bot, dist, dy, hasLOS, onGround)

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

func GenerateRetreatInputs(rng *rand.Rand, bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, target *PlayerInfo, botX, botY float64, objX, objY, objW, objH float64, physics PhysicsInfo, space *resolv.Space, teammates []PlayerInfo) {
	if target == nil {
		return
	}

	dx := target.X - botX
	dy := target.Y - botY
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

	switch {
	case dist < 60:
		// Too close - back away with random dodges
		if dx > 0 {
			intendedMoveLeft = true
		} else {
			intendedMoveRight = true
		}
		// Panic jump when very close
		if dist < 35 && physics.OnGround && bot.JumpCooldown <= 0 {
			input.CurrentInput[cfg.ActionJump] = true
			bot.JumpCooldown = 40
		}
	case dist < 150:
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
	default:
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
		if blocker := IsTeammateBlocking(botX, botY, movingRight, teammates); blocker != nil {
			// Jump over teammate
			if physics.OnGround && bot.JumpCooldown <= 0 {
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
	if physics.OnGround && bot.JumpCooldown <= 0 && (movingRight || movingLeft) {
		if gapDetected, gapWidth := DetectGapAhead(space, objX, objY, objW, objH, movingRight); gapDetected && gapWidth < cfg.Pathfinding.MaxJumpDistance {
			input.CurrentInput[cfg.ActionJump] = true
			bot.JumpCooldown = 20
		}
	}

	// Jump to reach target on higher platforms
	if dy < -60 && physics.OnGround && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 45
	}

	// Evasive jumps - occasionally jump to be unpredictable
	if physics.OnGround && bot.JumpCooldown <= 0 && rng.Float32() < 0.02 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 50
	}

	// Wall jump to escape - longer cooldown to prevent rapid bouncing
	if physics.WallSliding && bot.JumpCooldown <= 0 {
		input.CurrentInput[cfg.ActionJump] = true
		bot.JumpCooldown = 45
	}

	// Aggressive counter-attacks
	if bot.AttackCooldown <= 0 {
		switch {
		case dist > 80 && dist < 250:
			// Boomerang at range
			input.CurrentInput[cfg.ActionBoomerang] = true
			bot.AttackCooldown = 60
		case dist < 80 && dist > 50:
			// Jump kick to close distance aggressively
			if physics.OnGround && rng.Float32() < 0.4 {
				input.CurrentInput[cfg.ActionJump] = true
				input.CurrentInput[cfg.ActionAttack] = true
				bot.AttackCooldown = 45
				bot.JumpCooldown = 30
			} else {
				input.CurrentInput[cfg.ActionAttack] = true
				bot.AttackCooldown = 25
			}
		case dist <= 50:
			// Melee range - punch
			input.CurrentInput[cfg.ActionAttack] = true
			bot.AttackCooldown = 20
		}
	}
}

func DetectGapAhead(space *resolv.Space, objX, objY, objW, objH float64, movingRight bool) (bool, float64) {
	if space == nil {
		return false, 0
	}

	checkDist := cfg.Pathfinding.GapCheckDist
	checkDepth := cfg.Pathfinding.GapCheckDepth
	maxScanDist := cfg.Pathfinding.MaxJumpDistance
	scanStep := cfg.Pathfinding.LOSStepSize

	startY := objY + objH

	var startX float64
	if movingRight {
		startX = objX + objW + checkDist
	} else {
		startX = objX - checkDist
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
		scanX := objX + objW/2 + direction*dist

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

func HasLineOfSight(space *resolv.Space, x1, y1, x2, y2 float64) bool {
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

func ChooseAttack(rng *rand.Rand, bot *components.BotData, dist float64, dy float64, hasLOS bool, onGround bool) AttackChoice {
	var options []AttackChoice
	var weights []float64
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

func DetectIncomingThreats(botX, botY float64, botPlayerIndex int, botTeam int, players []PlayerInfo, boomerangs []BoomerangInfo) *ThreatInfo {
	foundThreat := false
	closestTime := 999
	var threat ThreatInfo

	getPlayerTeam := func(playerIndex int) int {
		for i := range players {
			if players[i].Index == playerIndex {
				return players[i].Team
			}
		}
		return -1
	}

	for _, b := range boomerangs {
		if b.OwnerIndex == botPlayerIndex {
			continue
		}

		if botTeam != -1 && getPlayerTeam(b.OwnerIndex) == botTeam {
			continue
		}

		dx := botX - b.X
		dy := botY - b.Y
		dist := mathutil.Distance(botX, botY, b.X, b.Y)

		dotProduct := dx*b.SpeedX + dy*b.SpeedY
		if dotProduct <= 0 {
			continue
		}

		speed := math.Sqrt(b.SpeedX*b.SpeedX + b.SpeedY*b.SpeedY)
		if speed < 0.1 {
			continue
		}
		timeToHit := int(dist / speed)

		if dist < 200 && timeToHit < closestTime && timeToHit < 60 {
			closestTime = timeToHit
			foundThreat = true
			threat.Type = ThreatBoomerang
			threat.X = b.X
			threat.Y = b.Y
			threat.VelX = b.SpeedX
			threat.VelY = b.SpeedY
			threat.TimeToHit = timeToHit
		}
	}

	for _, p := range players {
		if p.Index == botPlayerIndex {
			continue
		}

		if botTeam != -1 && p.Team == botTeam {
			continue
		}

		if p.CurrentState != cfg.Punch01 && p.CurrentState != cfg.Kick01 &&
			p.CurrentState != cfg.StateAttackingPunch && p.CurrentState != cfg.StateAttackingKick {
			continue
		}

		dist := mathutil.Distance(botX, botY, p.X, p.Y)

		if dist < 80 {
			timeToHit := 10
			if timeToHit < closestTime {
				closestTime = timeToHit
				foundThreat = true
				threat.Type = ThreatMelee
				threat.X = p.X
				threat.Y = p.Y
				threat.VelX = 0 // Inexact, but okay for melee
				threat.VelY = 0
				threat.TimeToHit = timeToHit
			}
		}
	}

	if foundThreat {
		return &threat
	}
	return nil
}

func GenerateDefensiveInputs(bot *components.BotData, input *components.PlayerInputData, player *components.PlayerData, threat *ThreatInfo, botX, botY float64, physics PhysicsInfo) bool {
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
	} else if threatComingLow && physics.OnGround && bot.JumpCooldown <= 0 {
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
