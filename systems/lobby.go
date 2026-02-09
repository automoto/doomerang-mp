package systems

import (
	"strings"

	"github.com/automoto/doomerang-mp/assets"
	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/hajimehoshi/ebiten/v2"
)

// InitLobby initializes a lobby with default settings
func InitLobby(lobby *components.LobbyData) {
	// Slot 0: Human with Scheme A (Arrows + Numpad)
	lobby.Slots[0] = components.PlayerSlot{
		Type:          components.SlotHuman,
		GamepadID:     nil,
		KeyboardZone:  components.KeyboardZoneArrows, // Kept for backwards compat
		ControlScheme: cfg.ControlSchemeA,
		Team:          -1,
		Ready:         false,
	}

	// Slot 1: Human with Scheme B (WASD + Space)
	lobby.Slots[1] = components.PlayerSlot{
		Type:          components.SlotHuman,
		GamepadID:     nil,
		KeyboardZone:  components.KeyboardZoneWASD, // Kept for backwards compat
		ControlScheme: cfg.ControlSchemeB,
		Team:          -1,
		Ready:         false,
	}

	// Slots 2-3: Empty by default
	lobby.Slots[2] = components.PlayerSlot{Type: components.SlotEmpty, Team: -1, KeyboardZone: components.KeyboardZoneNone}
	lobby.Slots[3] = components.PlayerSlot{Type: components.SlotEmpty, Team: -1, KeyboardZone: components.KeyboardZoneNone}

	// Default settings
	lobby.GameMode = cfg.GameModeFreeForAll
	lobby.MatchMinutes = 2

	// UI state
	lobby.SelectedSlot = 0
	lobby.SelectedOption = 0
	lobby.InSettings = false
	lobby.SettingsOption = 0
}

// GetActivePlayerCount returns the number of non-empty slots
func GetActivePlayerCount(lobby *components.LobbyData) int {
	count := 0
	for _, slot := range lobby.Slots {
		if slot.Type != components.SlotEmpty {
			count++
		}
	}
	return count
}

// GetHumanCount returns the number of human players
func GetHumanCount(lobby *components.LobbyData) int {
	count := 0
	for _, slot := range lobby.Slots {
		if slot.Type == components.SlotHuman {
			count++
		}
	}
	return count
}

// GetBotCount returns the number of bot players
func GetBotCount(lobby *components.LobbyData) int {
	count := 0
	for _, slot := range lobby.Slots {
		if slot.Type == components.SlotBot {
			count++
		}
	}
	return count
}

// CanStartMatch returns true if the match can be started
func CanStartMatch(lobby *components.LobbyData) bool {
	playerCount := GetActivePlayerCount(lobby)
	humanCount := GetHumanCount(lobby)

	// Need at least 1 human
	if humanCount < 1 {
		return false
	}

	// Game mode requirements
	switch lobby.GameMode {
	case cfg.GameMode1v1:
		return playerCount == 2
	case cfg.GameMode2v2:
		return playerCount == 4 && hasValidTeams(lobby)
	case cfg.GameModeFreeForAll:
		return playerCount >= 2
	case cfg.GameModeCoopVsBots:
		return humanCount >= 1 && GetBotCount(lobby) >= 1 && hasValidCoopTeams(lobby)
	}
	return false
}

// hasValidTeams checks if team assignments are valid for 2v2 mode
func hasValidTeams(lobby *components.LobbyData) bool {
	team0Count, team1Count := 0, 0
	for _, slot := range lobby.Slots {
		if slot.Type == components.SlotEmpty {
			continue
		}
		switch slot.Team {
		case 0:
			team0Count++
		case 1:
			team1Count++
		default:
			return false // Player not assigned to team
		}
	}
	return team0Count == 2 && team1Count == 2
}

// hasValidCoopTeams checks if team assignments are valid for Co-op mode
// Requires at least one player on each team
func hasValidCoopTeams(lobby *components.LobbyData) bool {
	team0Count, team1Count := 0, 0
	for _, slot := range lobby.Slots {
		if slot.Type == components.SlotEmpty {
			continue
		}
		switch slot.Team {
		case 0:
			team0Count++
		case 1:
			team1Count++
		default:
			return false // Player not assigned to team
		}
	}
	return team0Count >= 1 && team1Count >= 1
}

// CycleSlotType cycles the slot type (Empty -> Human -> Bot -> Empty)
func CycleSlotType(lobby *components.LobbyData, slotIndex int) {
	if slotIndex < 0 || slotIndex >= 4 {
		return
	}
	slot := &lobby.Slots[slotIndex]

	switch slot.Type {
	case components.SlotEmpty:
		slot.Type = components.SlotHuman
		// Try to assign an input device
		assignInputDevice(lobby, slotIndex)
	case components.SlotHuman:
		slot.Type = components.SlotBot
		slot.BotDifficulty = cfg.BotDifficultyNormal
		slot.GamepadID = nil
		slot.KeyboardZone = components.KeyboardZoneNone
	case components.SlotBot:
		slot.Type = components.SlotEmpty
		slot.GamepadID = nil
		slot.KeyboardZone = components.KeyboardZoneNone
	}
	// Re-assign teams when slot type changes (important for Co-op mode)
	AutoAssignTeams(lobby)
}

// CycleSlotTypeReverse cycles backwards (Empty <- Human <- Bot <- Empty)
func CycleSlotTypeReverse(lobby *components.LobbyData, slotIndex int) {
	if slotIndex < 0 || slotIndex >= 4 {
		return
	}
	slot := &lobby.Slots[slotIndex]

	switch slot.Type {
	case components.SlotEmpty:
		slot.Type = components.SlotBot
		slot.BotDifficulty = cfg.BotDifficultyNormal
	case components.SlotBot:
		slot.Type = components.SlotHuman
		assignInputDevice(lobby, slotIndex)
	case components.SlotHuman:
		slot.Type = components.SlotEmpty
		slot.GamepadID = nil
		slot.KeyboardZone = components.KeyboardZoneNone
	}
	// Re-assign teams when slot type changes (important for Co-op mode)
	AutoAssignTeams(lobby)
}

// CycleTeam cycles the team assignment for a slot
func CycleTeam(lobby *components.LobbyData, slotIndex int) {
	if slotIndex < 0 || slotIndex >= 4 {
		return
	}
	slot := &lobby.Slots[slotIndex]
	slot.Team = (slot.Team + 2) % 3 // -1 -> 0 -> 1 -> -1
	if slot.Team == 2 {
		slot.Team = -1
	}
}

// CycleBotDifficulty cycles bot difficulty
func CycleBotDifficulty(lobby *components.LobbyData, slotIndex int) {
	if slotIndex < 0 || slotIndex >= 4 {
		return
	}
	slot := &lobby.Slots[slotIndex]
	if slot.Type != components.SlotBot {
		return
	}
	slot.BotDifficulty = (slot.BotDifficulty + 1) % 3
}

// CycleControlScheme cycles the control scheme for a human slot
func CycleControlScheme(lobby *components.LobbyData, slotIndex int) {
	if slotIndex < 0 || slotIndex >= 4 {
		return
	}
	slot := &lobby.Slots[slotIndex]
	if slot.Type != components.SlotHuman || slot.GamepadID != nil {
		return
	}
	slot.ControlScheme = (slot.ControlScheme + 1) % cfg.ControlSchemeCount
	// Update KeyboardZone for backwards compat
	if slot.ControlScheme == cfg.ControlSchemeA {
		slot.KeyboardZone = components.KeyboardZoneArrows
	} else {
		slot.KeyboardZone = components.KeyboardZoneWASD
	}
}

// assignInputDevice assigns an available input device to a slot
func assignInputDevice(lobby *components.LobbyData, slotIndex int) {
	slot := &lobby.Slots[slotIndex]

	// Check which control schemes are in use
	schemeAUsed, schemeBUsed := false, false
	for i, s := range lobby.Slots {
		if i == slotIndex || s.Type != components.SlotHuman {
			continue
		}
		if s.GamepadID == nil { // Only keyboard players use control schemes
			if s.ControlScheme == cfg.ControlSchemeA {
				schemeAUsed = true
			}
			if s.ControlScheme == cfg.ControlSchemeB {
				schemeBUsed = true
			}
		}
	}

	// Assign control scheme if available (prefer Scheme A first)
	if !schemeAUsed {
		slot.ControlScheme = cfg.ControlSchemeA
		slot.KeyboardZone = components.KeyboardZoneArrows // For backwards compat
		slot.GamepadID = nil
		return
	}
	if !schemeBUsed {
		slot.ControlScheme = cfg.ControlSchemeB
		slot.KeyboardZone = components.KeyboardZoneWASD // For backwards compat
		slot.GamepadID = nil
		return
	}

	// Try to assign a gamepad
	for _, gp := range lobby.DetectedGamepads {
		gpUsed := false
		for i, s := range lobby.Slots {
			if i == slotIndex || s.Type != components.SlotHuman {
				continue
			}
			if s.GamepadID != nil && *s.GamepadID == gp {
				gpUsed = true
				break
			}
		}
		if !gpUsed {
			gpCopy := gp
			slot.GamepadID = &gpCopy
			slot.KeyboardZone = components.KeyboardZoneNone
			return
		}
	}

	// No input available, leave empty
	slot.KeyboardZone = components.KeyboardZoneNone
	slot.GamepadID = nil
}

// UpdateDetectedGamepads refreshes the list of connected gamepads
func UpdateDetectedGamepads(lobby *components.LobbyData) {
	lobby.DetectedGamepads = ebiten.AppendGamepadIDs(lobby.DetectedGamepads[:0])
}

// CycleGameMode cycles through available game modes
func CycleGameMode(lobby *components.LobbyData) {
	lobby.GameMode = (lobby.GameMode + 1) % 4
	// Auto-assign teams when switching to team-based modes
	AutoAssignTeams(lobby)
}

// AutoAssignTeams automatically assigns teams based on game mode
func AutoAssignTeams(lobby *components.LobbyData) {
	switch lobby.GameMode {
	case cfg.GameModeFreeForAll, cfg.GameMode1v1:
		// No teams - set all to -1
		for i := range lobby.Slots {
			lobby.Slots[i].Team = -1
		}
	case cfg.GameMode2v2:
		// Slots 0,1 = Team 0 (Red), Slots 2,3 = Team 1 (Blue)
		for i := range lobby.Slots {
			if i < 2 {
				lobby.Slots[i].Team = 0
			} else {
				lobby.Slots[i].Team = 1
			}
		}
	case cfg.GameModeCoopVsBots:
		// Humans = Team 0, Bots = Team 1
		for i := range lobby.Slots {
			switch lobby.Slots[i].Type {
			case components.SlotHuman:
				lobby.Slots[i].Team = 0
			case components.SlotBot:
				lobby.Slots[i].Team = 1
			default:
				lobby.Slots[i].Team = -1
			}
		}
	}
}

// CycleMatchTime cycles match duration (1-5 minutes)
func CycleMatchTime(lobby *components.LobbyData) {
	lobby.MatchMinutes++
	if lobby.MatchMinutes > 5 {
		lobby.MatchMinutes = 1
	}
}

// GetGameModeName returns a display name for the game mode
func GetGameModeName(mode cfg.GameModeID) string {
	switch mode {
	case cfg.GameModeFreeForAll:
		return "Free For All"
	case cfg.GameMode1v1:
		return "1 vs 1"
	case cfg.GameMode2v2:
		return "2 vs 2"
	case cfg.GameModeCoopVsBots:
		return "Co-op vs Bots"
	default:
		return "Unknown"
	}
}

// GetSlotTypeName returns a display name for slot type
func GetSlotTypeName(slotType components.PlayerSlotType) string {
	switch slotType {
	case components.SlotEmpty:
		return "Empty"
	case components.SlotHuman:
		return "Human"
	case components.SlotBot:
		return "Bot"
	default:
		return "Unknown"
	}
}

// GetBotDifficultyName returns a display name for bot difficulty
func GetBotDifficultyName(diff cfg.BotDifficulty) string {
	switch diff {
	case cfg.BotDifficultyEasy:
		return "Easy"
	case cfg.BotDifficultyNormal:
		return "Normal"
	case cfg.BotDifficultyHard:
		return "Hard"
	default:
		return "Unknown"
	}
}

// GetInputDeviceName returns a display name for the input device
func GetInputDeviceName(slot *components.PlayerSlot) string {
	if slot.Type == components.SlotBot {
		return "AI"
	}
	if slot.Type == components.SlotEmpty {
		return "-"
	}
	if slot.GamepadID != nil {
		return "Gamepad"
	}
	// Show control scheme name
	if int(slot.ControlScheme) < len(cfg.ControlSchemeNames) {
		return cfg.ControlSchemeNames[slot.ControlScheme]
	}
	return "None"
}

// GetTeamName returns a display name for team
func GetTeamName(team int) string {
	switch team {
	case 0:
		return "Red"
	case 1:
		return "Blue"
	default:
		return "None"
	}
}

// InitLevelList populates LevelNames from the embedded assets.
func InitLevelList(lobby *components.LobbyData) {
	loader := assets.NewLevelLoader()
	lobby.LevelNames = loader.ListLevelNames()
	lobby.LevelIndex = 0
}

// CycleLevel advances to the next level.
func CycleLevel(lobby *components.LobbyData) {
	if len(lobby.LevelNames) == 0 {
		return
	}
	lobby.LevelIndex = (lobby.LevelIndex + 1) % len(lobby.LevelNames)
}

// GetLevelDisplayName converts a stem name like "arena_battle_starter" to "Arena Battle Starter".
func GetLevelDisplayName(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
