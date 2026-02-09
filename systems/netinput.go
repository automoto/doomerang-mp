package systems

import (
	"log"
	"time"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi/ecs"
)

const resendInterval = 100 * time.Millisecond

type netInputState struct {
	seq            uint32
	lastDirection  int
	lastActions    map[cfg.ActionID]bool
	lastSendTime   time.Time
	currentActions map[cfg.ActionID]bool // reused each tick to avoid allocation
}

// NewNetworkInputSystem returns an ECS system that polls keyboard input (WASD scheme)
// and sends PlayerInput messages to the server when the input state changes.
func NewNetworkInputSystem(sendFn func(any) error) func(*ecs.ECS) {
	state := &netInputState{
		lastActions:    make(map[cfg.ActionID]bool),
		currentActions: make(map[cfg.ActionID]bool),
	}

	bindings := cfg.ControlSchemeBindings[cfg.ControlSchemeB]

	return func(_ *ecs.ECS) {
		dir := 0
		leftPressed := anyKeyPressed(bindings[cfg.ActionMoveLeft])
		rightPressed := anyKeyPressed(bindings[cfg.ActionMoveRight])
		if leftPressed && !rightPressed {
			dir = -1
		} else if rightPressed && !leftPressed {
			dir = 1
		}

		actions := state.currentActions
		actions[cfg.ActionJump] = anyKeyPressed(bindings[cfg.ActionJump])
		actions[cfg.ActionAttack] = anyKeyPressed(bindings[cfg.ActionAttack])
		actions[cfg.ActionBoomerang] = anyKeyPressed(bindings[cfg.ActionBoomerang])
		actions[cfg.ActionCrouch] = anyKeyPressed(bindings[cfg.ActionCrouch])

		changed := dir != state.lastDirection
		if !changed {
			for action, pressed := range actions {
				if pressed != state.lastActions[action] {
					changed = true
					break
				}
			}
		}

		now := time.Now()
		if !changed && now.Sub(state.lastSendTime) < resendInterval {
			return
		}

		state.seq++
		input := messages.NewPlayerInput(state.seq)
		input.Direction = dir
		for k, v := range actions {
			input.Actions[k] = v
		}
		input.Timestamp = now.UnixMilli()

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

func anyKeyPressed(keys []ebiten.Key) bool {
	for _, k := range keys {
		if ebiten.IsKeyPressed(k) {
			return true
		}
	}
	return false
}
