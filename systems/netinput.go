package systems

import (
	"log"
	"math"
	"time"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/leap-fish/necs/esync"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

const resendInterval = 50 * time.Millisecond

type netInputState struct {
	seq            uint32
	lastDirection  int
	lastActions    map[netconfig.ActionID]bool
	lastSendTime   time.Time
	currentActions map[netconfig.ActionID]bool // reused each tick to avoid allocation
}

// NewNetworkInputSystem returns an ECS system that polls keyboard input,
// applies it locally for prediction, and sends PlayerInput messages to the
// server when the input state changes.
func NewNetworkInputSystem(sendFn func(any) error, prediction *NetPrediction, localNetID func() esync.NetworkId) func(*ecs.ECS) {
	state := &netInputState{
		lastActions:    make(map[netconfig.ActionID]bool),
		currentActions: make(map[netconfig.ActionID]bool),
	}

	bindings := cfg.ControlSchemeBindings[cfg.ControlSchemeB]

	return func(e *ecs.ECS) {
		dir := 0
		leftPressed := anyKeyPressed(bindings[cfg.ActionMoveLeft])
		rightPressed := anyKeyPressed(bindings[cfg.ActionMoveRight])
		if leftPressed && !rightPressed {
			dir = -1
		} else if rightPressed && !leftPressed {
			dir = 1
		}

		actions := state.currentActions
		actions[netconfig.ActionJump] = anyKeyPressed(bindings[cfg.ActionJump])
		actions[netconfig.ActionAttack] = anyKeyPressed(bindings[cfg.ActionAttack])
		actions[netconfig.ActionBoomerang] = anyKeyPressed(bindings[cfg.ActionBoomerang])
		actions[netconfig.ActionCrouch] = anyKeyPressed(bindings[cfg.ActionCrouch])
		actions[netconfig.ActionMoveUp] = anyKeyPressed(bindings[cfg.ActionMoveUp])

		changed := dir != state.lastDirection
		if !changed {
			for action, pressed := range actions {
				if pressed != state.lastActions[action] {
					changed = true
					break
				}
			}
		}

		// Build the input message (needed for both prediction and sending)
		state.seq++
		input := messages.NewPlayerInput(state.seq)
		input.Direction = dir
		for k, v := range actions {
			input.Actions[k] = v
		}
		input.Timestamp = time.Now().UnixMilli()

		// Apply prediction locally every frame
		applyPrediction(e.World, prediction, input, localNetID())

		// Only send to server when input changes or resend interval elapses
		now := time.Now()
		if !changed && now.Sub(state.lastSendTime) < resendInterval {
			return
		}

		if err := sendFn(input); err != nil {
			log.Printf("[netinput] send error: %v", err)
		}

		state.lastDirection = dir
		for k, v := range actions {
			state.lastActions[k] = v
		}
		state.lastSendTime = now
	}
}

// applyPrediction finds the local player entity and runs one prediction step.
func applyPrediction(world donburi.World, pred *NetPrediction, input messages.PlayerInput, localID esync.NetworkId) {
	if pred == nil || localID == 0 {
		return
	}

	entity := esync.FindByNetworkId(world, localID)
	if !world.Valid(entity) {
		return
	}
	entry := world.Entry(entity)
	if !entry.HasComponent(netcomponents.NetPosition) {
		return
	}

	pos := netcomponents.NetPosition.Get(entry)
	pred.PredictStep(input, pos)

	if !entry.HasComponent(netcomponents.NetPlayerState) {
		return
	}
	state := netcomponents.NetPlayerState.Get(entry)
	if input.Direction != 0 {
		state.Direction = input.Direction
	}
	// Don't overwrite server-locked states
	if state.StateID == netconfig.Throw || state.StateID == netconfig.Hit {
		return
	}
	if input.Actions[netconfig.ActionBoomerang] {
		state.StateID = netconfig.StateChargingBoomerang
		return
	}
	if !pred.OnGround {
		state.StateID = netconfig.Jump
	} else if math.Abs(pred.VelX) >= 0.1 {
		state.StateID = netconfig.Running
	} else {
		state.StateID = netconfig.Idle
	}
}

func anyKeyPressed(keys []ebiten.Key) bool {
	for _, k := range keys {
		if ebiten.IsKeyPressed(k) {
			return true
		}
	}
	return false
}
