package netcomponents

import "github.com/yohamta/donburi"

type MatchStateID int

const (
	MatchStateWaiting MatchStateID = iota
	MatchStateCountdown
	MatchStatePlaying
	MatchStateRoundEnd
	MatchStateFinished
)

type NetGameStateData struct {
	MatchState    MatchStateID
	TimeRemaining float64
	Scores        map[uint32]int // NetworkId -> KO count
	Deaths        map[uint32]int // NetworkId -> death count
	WinnerID      uint32         // 0 if no winner yet
	GameMode      string         // "ffa", "1v1", "2v2"

	// Round system
	CurrentRound int
	RoundWins    map[int]int     // team number -> rounds won
	Lives        map[uint32]int  // NetworkId -> remaining lives
	Eliminated   map[uint32]bool // NetworkId -> eliminated this round
	RoundsToWin  int

	// Slot info for HUD positioning
	SlotNetIDs [4]uint32
	SlotNames  [4]string
	SlotTypes  [4]int // 0=Empty, 1=Human, 2=Bot
	SlotTeams  [4]int
}

var NetGameState = donburi.NewComponentType[NetGameStateData]()
