package components

import (
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
)

// InputMethod represents the type of input device being used
type InputMethod int

const (
	InputKeyboard InputMethod = iota
	InputXbox
	InputPlayStation
)

// KeyboardZone constants for split keyboard support
const (
	KeyboardZoneNone   = -1 // No keyboard (gamepad only)
	KeyboardZoneWASD   = 0  // WASD area for player 1/3
	KeyboardZoneArrows = 1  // Arrow area for player 2/4
)

// ActionState represents the temporal state of an action
type ActionState struct {
	Pressed      bool // Currently held down
	JustPressed  bool // Pressed this frame
	JustReleased bool // Released this frame
}

// InputData stores the current and previous frame's pressed state for all actions.
// JustPressed/JustReleased are computed on-demand by comparing frames.
// Used for global/menu input where all devices are merged.
type InputData struct {
	Current         [cfg.ActionCount]bool // Current frame's Pressed state
	Previous        [cfg.ActionCount]bool // Previous frame's Pressed state
	LastInputMethod InputMethod           // Most recently used input method
}

var Input = donburi.NewComponentType[InputData]()

// PlayerInputData stores per-player input state.
// Each player entity has their own PlayerInputData with a bound input device.
type PlayerInputData struct {
	PlayerIndex    int                   // 0-3 player index
	CurrentInput   [cfg.ActionCount]bool // Current frame's Pressed state
	PreviousInput  [cfg.ActionCount]bool // Previous frame's Pressed state
	BoundGamepadID *ebiten.GamepadID     // Bound gamepad (nil = keyboard)
	KeyboardZone   int                   // DEPRECATED: use ControlScheme instead
	ControlScheme  cfg.ControlSchemeID   // Active control scheme (A=Arrows+Numpad, B=WASD+Space)
	InputMethod    InputMethod           // Current input method (for UI prompts)
}

var PlayerInput = donburi.NewComponentType[PlayerInputData]()
