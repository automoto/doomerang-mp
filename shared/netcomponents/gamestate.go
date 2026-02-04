package netcomponents

import "github.com/yohamta/donburi"

type MatchState int

const (
	MatchStateWaiting MatchState = iota
	MatchStateCountdown
	MatchStatePlaying
	MatchStateFinished
)

type NetGameStateData struct {
	Scores     map[uint]int // NetworkId -> score
	Timer      float64      // Remaining time or elapsed
	MatchState MatchState
}

var NetGameState = donburi.NewComponentType[NetGameStateData]()
