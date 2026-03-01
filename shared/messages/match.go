package messages

import "github.com/automoto/doomerang-mp/shared/netcomponents"

// MatchEvent is broadcast for match flow transitions
type MatchEvent struct {
	Type        string // "countdown_start", "match_start", "match_end", "round_end", "player_eliminated"
	Message     string
	WinnerID    uint32
	Reason      string
	Scores      map[uint32]int
	RoundNumber int    // Which round (for "round_end", "player_eliminated")
	PlayerID    uint32 // Eliminated player (for "player_eliminated")
}

// ScoreEvent is broadcast when a player's score changes
type ScoreEvent struct {
	PlayerID uint32
	KOs      int
	Deaths   int
}

// CreateRoomRequest is sent by a client to create a private or public room
type CreateRoomRequest struct {
	GameMode   string
	Password   string // Empty for public
	MaxPlayers int
}

// RoomCreated is sent by the server after successfully creating a room
type RoomCreated struct {
	RoomID   string
	RoomCode string
}

// JoinRoomRequest is sent by a client to join a specific room via code
type JoinRoomRequest struct {
	RoomCode string
	Password string
}

// LobbyAction represents an action taken in the lobby (picking slot, readying up, etc.)
type LobbyAction struct {
	Action string // "pick_slot", "ready", "unready", "change_mode", "change_time", "change_level", "add_bot", "remove_bot"
	Value  int    // Slot index, or value for the action
	String string // For actions requiring string values
}

// LobbyUpdate is broadcast when the lobby state changes
type LobbyUpdate struct {
	Slots        [4]LobbySlot
	GameMode     string
	MatchMinutes int
	LevelIndex   int
	HostID       uint32
}

type LobbySlot struct {
	Type      int    // 0=Empty, 1=Human, 2=Bot
	PlayerID  uint32 // NetworkID of human player, 0 if empty/bot
	Ready     bool
	Team      int
	Difficulty int
	Name      string
}

// SyncGameState is sent periodically or on change to ensure clients have latest info
type SyncGameState struct {
	State netcomponents.NetGameStateData
}
