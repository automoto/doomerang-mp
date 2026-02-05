package systems

import (
	"strings"

	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

// Reusable slice for gamepad IDs to avoid allocations
var gamepadIDs []ebiten.GamepadID

// Cache controller types to avoid string allocation every frame
var controllerTypeCache = make(map[ebiten.GamepadID]components.InputMethod)

// UpdateInput polls raw input and updates the InputComponent.
// Must run BEFORE UpdatePlayer in the system order.
func UpdateInput(ecs *ecs.ECS) {
	input := getOrCreateInput(ecs)

	// Swap buffers: current becomes previous, then zero out current
	input.Previous = input.Current
	input.Current = [cfg.ActionCount]bool{}

	// Get connected gamepads
	gamepadIDs = ebiten.AppendGamepadIDs(gamepadIDs[:0])

	// Read analog stick state (with deadzone)
	analogLeft, analogRight, analogUp, analogDown, analogGpID := getAnalogStickState(gamepadIDs)

	// Track which input method was used this frame
	var keyboardUsed, gamepadUsed bool
	var activeGamepadID ebiten.GamepadID

	// Poll all actions - only set Pressed state
	for actionID, binding := range cfg.Input.Bindings {
		// Check keyboard keys
		for _, key := range binding.Keys {
			if ebiten.IsKeyPressed(key) {
				input.Current[actionID] = true
				keyboardUsed = true
			}
		}

		// Check gamepad buttons
		for _, gpID := range gamepadIDs {
			if !ebiten.IsStandardGamepadLayoutAvailable(gpID) {
				continue
			}
			for _, btn := range binding.StandardGamepadButtons {
				if ebiten.IsStandardGamepadButtonPressed(gpID, btn) {
					input.Current[actionID] = true
					gamepadUsed = true
					activeGamepadID = gpID
				}
			}
		}
	}

	// Merge analog stick into directional actions
	if analogLeft {
		input.Current[cfg.ActionMoveLeft] = true
		input.Current[cfg.ActionMenuLeft] = true
		gamepadUsed = true
		activeGamepadID = analogGpID
	}
	if analogRight {
		input.Current[cfg.ActionMoveRight] = true
		input.Current[cfg.ActionMenuRight] = true
		gamepadUsed = true
		activeGamepadID = analogGpID
	}
	if analogUp {
		input.Current[cfg.ActionMoveUp] = true
		input.Current[cfg.ActionMenuUp] = true
		gamepadUsed = true
		activeGamepadID = analogGpID
	}
	if analogDown {
		input.Current[cfg.ActionCrouch] = true
		input.Current[cfg.ActionMenuDown] = true
		gamepadUsed = true
		activeGamepadID = analogGpID
	}

	// Update last input method - gamepad takes priority if both used
	if gamepadUsed {
		input.LastInputMethod = getControllerType(activeGamepadID)
	} else if keyboardUsed {
		input.LastInputMethod = components.InputKeyboard
	}
}

// getControllerType returns cached controller type, detecting on first access
func getControllerType(gpID ebiten.GamepadID) components.InputMethod {
	if method, ok := controllerTypeCache[gpID]; ok {
		return method
	}

	// Detect and cache controller type
	name := strings.ToLower(ebiten.GamepadName(gpID))
	var method components.InputMethod
	if strings.Contains(name, "ps4") || strings.Contains(name, "ps5") ||
		strings.Contains(name, "playstation") || strings.Contains(name, "dualshock") ||
		strings.Contains(name, "dualsense") {
		method = components.InputPlayStation
	} else {
		// Default gamepad to Xbox-style
		method = components.InputXbox
	}

	controllerTypeCache[gpID] = method
	return method
}

// getAnalogStickState reads the left analog stick from all gamepads
// Returns directional states based on deadzone threshold and the active gamepad ID
func getAnalogStickState(gamepads []ebiten.GamepadID) (left, right, up, down bool, activeGpID ebiten.GamepadID) {
	deadzone := cfg.Input.AnalogDeadzone

	for _, gpID := range gamepads {
		if !ebiten.IsStandardGamepadLayoutAvailable(gpID) {
			continue
		}

		// Read left stick axes
		horizontal := ebiten.StandardGamepadAxisValue(gpID, ebiten.StandardGamepadAxisLeftStickHorizontal)
		vertical := ebiten.StandardGamepadAxisValue(gpID, ebiten.StandardGamepadAxisLeftStickVertical)

		// Apply deadzone
		if horizontal < -deadzone {
			left = true
			activeGpID = gpID
		}
		if horizontal > deadzone {
			right = true
			activeGpID = gpID
		}
		if vertical < -deadzone {
			up = true
			activeGpID = gpID
		}
		if vertical > deadzone {
			down = true
			activeGpID = gpID
		}
	}

	return
}

// getOrCreateInput returns the singleton Input component, creating if needed
func getOrCreateInput(ecs *ecs.ECS) *components.InputData {
	entry, ok := components.Input.First(ecs.World)
	if !ok {
		entry = ecs.World.Entry(ecs.World.Create(components.Input))
		// Zero-value InputData is correct (all bools false)
	}
	return components.Input.Get(entry)
}

// GetAction returns the full ActionState for an action ID.
// JustPressed/JustReleased are derived from current vs previous frame.
func GetAction(input *components.InputData, id cfg.ActionID) components.ActionState {
	curr := input.Current[id]
	prev := input.Previous[id]
	return components.ActionState{
		Pressed:      curr,
		JustPressed:  curr && !prev,
		JustReleased: !curr && prev,
	}
}

// UpdateMultiPlayerInput polls input for all player entities with PlayerInputData.
// Must run AFTER UpdateInput (which handles global/menu input).
func UpdateMultiPlayerInput(ecs *ecs.ECS) {
	gamepadIDs = ebiten.AppendGamepadIDs(gamepadIDs[:0])

	components.PlayerInput.Each(ecs.World, func(entry *donburi.Entry) {
		input := components.PlayerInput.Get(entry)
		updatePlayerInputData(input, gamepadIDs)
	})
}

// updatePlayerInputData polls input for a single player based on their bound device.
func updatePlayerInputData(input *components.PlayerInputData, gamepads []ebiten.GamepadID) {
	input.PreviousInput = input.CurrentInput
	input.CurrentInput = [cfg.ActionCount]bool{}

	if input.BoundGamepadID != nil {
		pollGamepadForPlayer(input, *input.BoundGamepadID)
		return
	}

	// Use ControlScheme for keyboard input
	if input.ControlScheme >= 0 && int(input.ControlScheme) < len(cfg.ControlSchemeBindings) {
		pollControlSchemeForPlayer(input, input.ControlScheme)
	}
}

// pollGamepadForPlayer reads input from a specific gamepad into PlayerInputData.
func pollGamepadForPlayer(input *components.PlayerInputData, gpID ebiten.GamepadID) {
	if !ebiten.IsStandardGamepadLayoutAvailable(gpID) {
		return
	}

	for actionID, binding := range cfg.Input.Bindings {
		for _, btn := range binding.StandardGamepadButtons {
			if ebiten.IsStandardGamepadButtonPressed(gpID, btn) {
				input.CurrentInput[actionID] = true
				input.InputMethod = getControllerType(gpID)
			}
		}
	}

	deadzone := cfg.Input.AnalogDeadzone
	horizontal := ebiten.StandardGamepadAxisValue(gpID, ebiten.StandardGamepadAxisLeftStickHorizontal)
	vertical := ebiten.StandardGamepadAxisValue(gpID, ebiten.StandardGamepadAxisLeftStickVertical)

	if horizontal < -deadzone {
		input.CurrentInput[cfg.ActionMoveLeft] = true
		input.CurrentInput[cfg.ActionMenuLeft] = true
		input.InputMethod = getControllerType(gpID)
	}
	if horizontal > deadzone {
		input.CurrentInput[cfg.ActionMoveRight] = true
		input.CurrentInput[cfg.ActionMenuRight] = true
		input.InputMethod = getControllerType(gpID)
	}
	if vertical < -deadzone {
		input.CurrentInput[cfg.ActionMoveUp] = true
		input.CurrentInput[cfg.ActionMenuUp] = true
		input.InputMethod = getControllerType(gpID)
	}
	if vertical > deadzone {
		input.CurrentInput[cfg.ActionCrouch] = true
		input.CurrentInput[cfg.ActionMenuDown] = true
		input.InputMethod = getControllerType(gpID)
	}
}

// pollControlSchemeForPlayer reads input from a control scheme into PlayerInputData.
func pollControlSchemeForPlayer(input *components.PlayerInputData, scheme cfg.ControlSchemeID) {
	schemeBindings := cfg.ControlSchemeBindings[scheme]
	keyPressed := false

	for actionID, keys := range schemeBindings {
		for _, key := range keys {
			if ebiten.IsKeyPressed(key) {
				input.CurrentInput[actionID] = true
				keyPressed = true
			}
		}
	}

	if keyPressed {
		input.InputMethod = components.InputKeyboard
	}
}

// GetPlayerAction returns the full ActionState for an action ID from PlayerInputData.
func GetPlayerAction(input *components.PlayerInputData, id cfg.ActionID) components.ActionState {
	curr := input.CurrentInput[id]
	prev := input.PreviousInput[id]
	return components.ActionState{
		Pressed:      curr,
		JustPressed:  curr && !prev,
		JustReleased: !curr && prev,
	}
}
