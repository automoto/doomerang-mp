package core

import (
	"math/rand"

	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/botai"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/automoto/doomerang-mp/shared/pathfinding"
	"github.com/yohamta/donburi"
)

type BotSystem struct {
	server *Server
	rng    *rand.Rand

	cachedNavGrid *pathfinding.NavGrid
	levelName     string
}

func NewBotSystem(server *Server) *BotSystem {
	return &BotSystem{
		server: server,
		rng:    rand.New(rand.NewSource(42)),
	}
}

func (s *BotSystem) Update() {
	world := s.server.world
	level := s.server.activeLevel

	if level == nil {
		return
	}

	navGrid := s.getOrCreateNavGrid(level)

	var players []botai.PlayerInfo
	netcomponents.NetPlayerState.Each(world, func(entry *donburi.Entry) {
		pos := netcomponents.NetPosition.Get(entry)
		state := netcomponents.NetPlayerState.Get(entry)

		playerIndex := -1
		if entry.HasComponent(components.Player) {
			playerIndex = components.Player.Get(entry).PlayerIndex
		}

		players = append(players, botai.PlayerInfo{
			Index:        playerIndex,
			X:            pos.X + 8, // Center (assuming 16x40)
			Y:            pos.Y + 20,
			W:            16,
			H:            40,
			Health:       state.Health,
			MaxHealth:    100,
			IsBot:        state.IsBot,
			Team:         -1, // TODO: team support on server
			CurrentState: netconfigToStateID(state.StateID),
		})
	})

	var boomerangs []botai.BoomerangInfo
	// TODO: Get boomerangs from server world/physics

	components.Bot.Each(world, func(entry *donburi.Entry) {
		bot := components.Bot.Get(entry)
		input := components.PlayerInput.Get(entry)
		player := components.Player.Get(entry)
		pos := netcomponents.NetPosition.Get(entry)
		vel := netcomponents.NetVelocity.Get(entry)

		// Get physics info from PlayerPhysics
		physicsInfo := botai.PhysicsInfo{
			OnGround:    false,
			WallSliding: false,
			SpeedX:      vel.SpeedX,
			SpeedY:      vel.SpeedY,
		}

		if pp, ok := s.server.playerPhysics[entry.Entity()]; ok {
			physicsInfo.OnGround = pp.OnGround
		}

		botai.UpdateBotAI(
			s.rng,
			bot,
			input,
			player,
			pos.X, pos.Y, 16, 40,
			100, 100, // TODO: Health
			physicsInfo,
			players,
			boomerangs,
			level.Space,
			navGrid,
		)

		// Apply inputs to PlayerPhysics
		if pp, ok := s.server.playerPhysics[entry.Entity()]; ok {
			pp.Direction = 0
			if input.CurrentInput[cfg.ActionMoveRight] {
				pp.Direction = 1
			} else if input.CurrentInput[cfg.ActionMoveLeft] {
				pp.Direction = -1
			}

			pp.JumpPressed = input.CurrentInput[cfg.ActionJump]
			pp.BoomerangPressed = input.CurrentInput[cfg.ActionBoomerang]
			pp.MoveUpPressed = input.CurrentInput[cfg.ActionMoveUp]
			pp.CrouchPressed = input.CurrentInput[cfg.ActionCrouch]

			// Update facing direction in NetPlayerState
			if pp.Direction != 0 {
				state := netcomponents.NetPlayerState.Get(entry)
				state.Direction = pp.Direction
			}
		}
	})
}

func (s *BotSystem) getOrCreateNavGrid(level *ServerLevel) *pathfinding.NavGrid {
	// We need a name for the level to cache the navgrid.
	// Since ServerLevel doesn't have a name yet, let's just use the pointer for now or add a Name.
	// Wait, ServerLevel in server.go has names in the map.

	// For now, let's just always use the active level from the server.
	if s.cachedNavGrid != nil && s.levelName == s.server.activeName {
		return s.cachedNavGrid
	}

	s.cachedNavGrid = pathfinding.CreateNavGrid(level.Space, level.MapWidth, level.MapHeight, 32.0)
	s.levelName = s.server.activeName
	return s.cachedNavGrid
}

func netconfigToStateID(stateID netconfig.StateID) cfg.StateID {
	switch stateID {
	case netconfig.Idle:
		return cfg.Idle
	case netconfig.Running:
		return cfg.Running
	case netconfig.Jump:
		return cfg.Jump
	default:
		return cfg.Idle
	}
}
