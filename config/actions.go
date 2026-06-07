package config

import "github.com/automoto/doomerang-mp/shared/netconfig"

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
