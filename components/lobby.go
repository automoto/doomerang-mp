package components

import (
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
)

// PlayerSlotType identifies what kind of player is in a slot
type PlayerSlotType int

const (
	SlotEmpty PlayerSlotType = iota
	SlotHuman
	SlotBot
)

// PlayerSlot represents a player slot in the lobby
type PlayerSlot struct {
	Type          PlayerSlotType      // Empty, Human, or Bot
	GamepadID     *ebiten.GamepadID   // Assigned gamepad (nil = keyboard)
	KeyboardZone  int                 // Keyboard zone (0=WASD, 1=Arrows, -1=none) - DEPRECATED: use ControlScheme
	ControlScheme cfg.ControlSchemeID // Control scheme (A=Arrows+Numpad, B=WASD+Space)
	BotDifficulty cfg.BotDifficulty   // Difficulty if bot
	Team          int                 // Team index (-1 = no team/FFA)
	Ready         bool                // Player is ready to start
}

// LobbyData stores the lobby/match configuration state
type LobbyData struct {
	// Player configuration
	Slots [4]PlayerSlot // Up to 4 player slots

	// Match settings
	GameMode     cfg.GameModeID // FFA, 1v1, 2v2, CoopVsBots
	MatchMinutes int            // Match duration in minutes (1-10)
	LevelIndex   int            // Currently selected level index
	LevelNames   []string       // Available level stem names

	// UI state
	SelectedSlot   int  // Currently selected slot (0-3)
	SelectedOption int  // Current option within slot/settings (0=type, 1=team, etc.)
	InSettings     bool // True if in settings area vs player slots
	SettingsOption int  // Which setting is selected (0=mode, 1=time)

	// Detected gamepads
	DetectedGamepads []ebiten.GamepadID
}

var Lobby = donburi.NewComponentType[LobbyData]()
