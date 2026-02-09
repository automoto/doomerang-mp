package config

import (
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/hajimehoshi/ebiten/v2"
)

// Type alias — all existing client code using config.ActionID keeps working.
type ActionID = netconfig.ActionID

// Re-export action constants.
const (
	ActionNone       = netconfig.ActionNone
	ActionMoveLeft   = netconfig.ActionMoveLeft
	ActionMoveRight  = netconfig.ActionMoveRight
	ActionMoveUp     = netconfig.ActionMoveUp
	ActionJump       = netconfig.ActionJump
	ActionAttack     = netconfig.ActionAttack
	ActionCrouch     = netconfig.ActionCrouch
	ActionBoomerang  = netconfig.ActionBoomerang
	ActionPause      = netconfig.ActionPause
	ActionMenuUp     = netconfig.ActionMenuUp
	ActionMenuDown   = netconfig.ActionMenuDown
	ActionMenuLeft   = netconfig.ActionMenuLeft
	ActionMenuRight  = netconfig.ActionMenuRight
	ActionMenuSelect = netconfig.ActionMenuSelect
	ActionMenuBack   = netconfig.ActionMenuBack
	ActionCount      = netconfig.ActionCount
)

// ControlSchemeID identifies a control scheme preset
type ControlSchemeID int

const (
	ControlSchemeA ControlSchemeID = iota // Arrows + Numpad
	ControlSchemeB                        // WASD + Space
	ControlSchemeCount
)

// ControlSchemeNames for UI display
var ControlSchemeNames = [ControlSchemeCount]string{
	"Arrows+Numbers",
	"WASD+Space",
}

// InputBinding represents a single key or button binding for an action
type InputBinding struct {
	Keys                   []ebiten.Key
	StandardGamepadButtons []ebiten.StandardGamepadButton
}

// InputConfig holds all input mappings
type InputConfig struct {
	Bindings map[ActionID]InputBinding
	// Deadzone for analog stick input (0.0 to 1.0)
	AnalogDeadzone float64
}

// Input is the global input configuration
var Input InputConfig

// ControlSchemeBindings maps ActionID to keys for each control scheme.
// These replace the old "keyboard zones" concept with dedicated jump buttons.
var ControlSchemeBindings = [ControlSchemeCount]map[ActionID][]ebiten.Key{
	// Scheme A: Arrows + Number Keys
	{
		ActionMoveLeft:   {ebiten.KeyLeft},
		ActionMoveRight:  {ebiten.KeyRight},
		ActionMoveUp:     {ebiten.KeyUp},     // For aiming up
		ActionJump:       {ebiten.KeyDigit0}, // Dedicated jump button
		ActionAttack:     {ebiten.KeyDigit8},
		ActionCrouch:     {ebiten.KeyDown},
		ActionBoomerang:  {ebiten.KeyDigit9},
		ActionPause:      {ebiten.KeyEscape},
		ActionMenuUp:     {ebiten.KeyUp},
		ActionMenuDown:   {ebiten.KeyDown},
		ActionMenuLeft:   {ebiten.KeyLeft},
		ActionMenuRight:  {ebiten.KeyRight},
		ActionMenuSelect: {ebiten.KeyEnter, ebiten.KeyDigit0},
		ActionMenuBack:   {ebiten.KeyBackspace},
	},
	// Scheme B: WASD + Space/F/G
	{
		ActionMoveLeft:   {ebiten.KeyA},
		ActionMoveRight:  {ebiten.KeyD},
		ActionMoveUp:     {ebiten.KeyW},     // For aiming up
		ActionJump:       {ebiten.KeySpace}, // Dedicated jump button
		ActionAttack:     {ebiten.KeyF},
		ActionCrouch:     {ebiten.KeyS},
		ActionBoomerang:  {ebiten.KeyG},
		ActionPause:      {ebiten.KeyEscape},
		ActionMenuUp:     {ebiten.KeyW},
		ActionMenuDown:   {ebiten.KeyS},
		ActionMenuLeft:   {ebiten.KeyA},
		ActionMenuRight:  {ebiten.KeyD},
		ActionMenuSelect: {ebiten.KeySpace},
		ActionMenuBack:   {ebiten.KeyTab},
	},
}

// KeyboardZoneBindings is kept for backwards compatibility but maps to schemes.
// Zone 0 -> Scheme B (WASD), Zone 1 -> Scheme A (Arrows)
// This maintains existing behavior for code that uses zones.
var KeyboardZoneBindings = [2]map[ActionID][]ebiten.Key{
	ControlSchemeBindings[ControlSchemeB], // Zone 0 = WASD = Scheme B
	ControlSchemeBindings[ControlSchemeA], // Zone 1 = Arrows = Scheme A
}

func init() {
	Input = InputConfig{
		AnalogDeadzone: 0.25,
		Bindings: map[ActionID]InputBinding{
			ActionMoveLeft: {
				Keys: []ebiten.Key{ebiten.KeyLeft, ebiten.KeyA},
				// D-pad Left (analog stick handled separately)
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonLeftLeft,
				},
			},
			ActionMoveRight: {
				Keys: []ebiten.Key{ebiten.KeyRight, ebiten.KeyD},
				// D-pad Right (analog stick handled separately)
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonLeftRight,
				},
			},
			ActionMoveUp: {
				Keys: []ebiten.Key{ebiten.KeyUp, ebiten.KeyW},
				// D-pad Up (analog stick handled separately)
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonLeftTop,
				},
			},
			ActionJump: {
				Keys: []ebiten.Key{ebiten.KeyDigit0, ebiten.KeySpace},
				// A / Cross button
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonRightBottom,
				},
			},
			ActionAttack: {
				Keys: []ebiten.Key{ebiten.KeyDigit8, ebiten.KeyF},
				// B / Circle button
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonRightRight,
				},
			},
			ActionCrouch: {
				Keys: []ebiten.Key{ebiten.KeyDown, ebiten.KeyS},
				// D-pad Down (analog stick handled separately)
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonLeftBottom,
				},
			},
			ActionBoomerang: {
				Keys: []ebiten.Key{ebiten.KeyDigit9, ebiten.KeyG},
				// X / Square button
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonRightLeft,
				},
			},
			ActionPause: {
				Keys: []ebiten.Key{ebiten.KeyEscape, ebiten.KeyP},
				// Start / Options button
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonCenterRight,
				},
			},
			ActionMenuUp: {
				Keys: []ebiten.Key{ebiten.KeyUp, ebiten.KeyW},
				// D-pad Up (analog stick handled separately)
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonLeftTop,
				},
			},
			ActionMenuDown: {
				Keys: []ebiten.Key{ebiten.KeyDown, ebiten.KeyS},
				// D-pad Down (analog stick handled separately)
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonLeftBottom,
				},
			},
			ActionMenuLeft: {
				Keys: []ebiten.Key{ebiten.KeyLeft, ebiten.KeyA},
				// D-pad Left (analog stick handled separately)
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonLeftLeft,
				},
			},
			ActionMenuRight: {
				Keys: []ebiten.Key{ebiten.KeyRight, ebiten.KeyD},
				// D-pad Right (analog stick handled separately)
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonLeftRight,
				},
			},
			ActionMenuSelect: {
				Keys: []ebiten.Key{ebiten.KeyEnter, ebiten.KeyDigit0, ebiten.KeySpace},
				// A / Cross button
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonRightBottom,
				},
			},
			ActionMenuBack: {
				Keys: []ebiten.Key{ebiten.KeyEscape, ebiten.KeyBackspace},
				// B / Circle button
				StandardGamepadButtons: []ebiten.StandardGamepadButton{
					ebiten.StandardGamepadButtonRightRight,
				},
			},
		},
	}
}
