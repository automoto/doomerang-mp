package components

import (
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/yohamta/donburi"
)

// PlayerScore tracks a player's match statistics
type PlayerScore struct {
	PlayerIndex int
	KOs         int // Kills/knockouts
	Deaths      int
	Team        int // 0 or 1 for team modes, -1 for FFA
}

// MatchData stores the current match state and scores.
// This is a singleton component - only one match exists at a time.
type MatchData struct {
	State          cfg.MatchStateID
	GameMode       cfg.GameModeID
	Timer          int           // Countdown or match timer (frames remaining)
	Duration       int           // Total match duration (frames)
	Scores         []PlayerScore // Score per player slot (indexed by PlayerIndex)
	WinnerIndex    int           // PlayerIndex of winner (-1 if no winner yet, -2 for tie)
	WinningTeam    int           // Team index for team modes (-1 if not applicable)
	CountdownValue int           // Current countdown number (3, 2, 1, GO)
}

var Match = donburi.NewComponentType[MatchData]()

// GetPlayerScore returns the score for a player, creating it if needed
func (m *MatchData) GetPlayerScore(playerIndex int) *PlayerScore {
	// Ensure slice is large enough
	for len(m.Scores) <= playerIndex {
		m.Scores = append(m.Scores, PlayerScore{
			PlayerIndex: len(m.Scores),
			Team:        -1, // Default to no team (FFA)
		})
	}
	return &m.Scores[playerIndex]
}

// AddKO increments KO count for a player
func (m *MatchData) AddKO(playerIndex int) {
	score := m.GetPlayerScore(playerIndex)
	score.KOs++
}

// AddDeath increments death count for a player
func (m *MatchData) AddDeath(playerIndex int) {
	score := m.GetPlayerScore(playerIndex)
	score.Deaths++
}

// GetLeader returns the player index with the most KOs (-1 for tie, -2 for no scores)
func (m *MatchData) GetLeader() int {
	if len(m.Scores) == 0 {
		return -2
	}

	maxKOs := -1
	leader := -1
	tied := false

	for _, score := range m.Scores {
		if score.KOs > maxKOs {
			maxKOs = score.KOs
			leader = score.PlayerIndex
			tied = false
		} else if score.KOs == maxKOs && maxKOs >= 0 {
			tied = true
		}
	}

	if tied {
		return -1 // Tie
	}
	return leader
}

// GetTeamScore returns total KOs for a team
func (m *MatchData) GetTeamScore(team int) int {
	total := 0
	for _, score := range m.Scores {
		if score.Team == team {
			total += score.KOs
		}
	}
	return total
}
