package systems

import (
	"math/rand"

	"github.com/automoto/doomerang-mp/components"
	"github.com/automoto/doomerang-mp/shared/botai"
	"github.com/automoto/doomerang-mp/shared/pathfinding"
	"github.com/automoto/doomerang-mp/tags"
	"github.com/solarlune/resolv"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

var rng = rand.New(rand.NewSource(42)) // Fixed seed for deterministic replay

// Nav grid cache (created once per level)
var cachedNavGrid *pathfinding.NavGrid
var navGridLevelID string

// UpdateBots generates input for bot-controlled players.
// Must run BEFORE UpdateMultiPlayerInput.
func UpdateBots(e *ecs.ECS) {
	if !IsMatchPlaying(e) {
		return
	}

	// In network mode, the client doesn't run bot AI
	if isNetworkMode(e) {
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

	var players []botai.PlayerInfo
	tags.Player.Each(e.World, func(entry *donburi.Entry) {
		obj := components.Object.Get(entry)
		player := components.Player.Get(entry)
		health := components.Health.Get(entry)
		state := components.State.Get(entry)

		team := -1
		if match != nil {
			team = match.GetPlayerScore(player.PlayerIndex).Team
		}

		players = append(players, botai.PlayerInfo{
			Index:        player.PlayerIndex,
			X:            obj.X + obj.W/2,
			Y:            obj.Y + obj.H/2,
			W:            obj.W,
			H:            obj.H,
			Health:       health.Current,
			MaxHealth:    health.Max,
			IsBot:        entry.HasComponent(components.Bot),
			Team:         team,
			CurrentState: state.CurrentState,
		})
	})

	var boomerangs []botai.BoomerangInfo
	components.Boomerang.Each(e.World, func(entry *donburi.Entry) {
		b := components.Boomerang.Get(entry)
		obj := components.Object.Get(entry)
		physics := components.Physics.Get(entry)
		boomerangs = append(boomerangs, botai.BoomerangInfo{
			OwnerIndex: b.OwnerIndex,
			X:          obj.X,
			Y:          obj.Y,
			SpeedX:     physics.SpeedX,
			SpeedY:     physics.SpeedY,
		})
	})

	components.Bot.Each(e.World, func(entry *donburi.Entry) {
		bot := components.Bot.Get(entry)
		input := components.PlayerInput.Get(entry)
		player := components.Player.Get(entry)
		obj := components.Object.Get(entry)
		health := components.Health.Get(entry)
		physics := components.Physics.Get(entry)

		physInfo := botai.PhysicsInfo{
			OnGround:    physics.OnGround != nil,
			WallSliding: physics.WallSliding != nil,
			SpeedX:      physics.SpeedX,
			SpeedY:      physics.SpeedY,
		}

		botai.UpdateBotAI(
			rng,
			bot,
			input,
			player,
			obj.X, obj.Y, obj.W, obj.H,
			health.Current, health.Max,
			physInfo,
			players,
			boomerangs,
			space,
			navGrid,
		)
	})
}

func isNetworkMode(e *ecs.ECS) bool {
	if entry, ok := components.NetworkConfig.First(e.World); ok {
		return components.NetworkConfig.Get(entry).IsNetwork
	}
	return false
}

func getOrCreateNavGrid(e *ecs.ECS, space *resolv.Space) *pathfinding.NavGrid {
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

	cachedNavGrid = pathfinding.CreateNavGrid(space, levelData.CurrentLevel.Width, levelData.CurrentLevel.Height, 32.0)
	navGridLevelID = currentLevelID

	return cachedNavGrid
}
