package systems

import (
	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/yohamta/donburi/ecs"
)

// UpdateMatch handles match state transitions and timers.
func UpdateMatch(e *ecs.ECS) {
	matchEntry, ok := components.Match.First(e.World)
	if !ok {
		return
	}
	match := components.Match.Get(matchEntry)

	switch match.State {
	case cfg.MatchStateWaiting:
		// Waiting state is handled externally (lobby)
		// For now, auto-start when match is created
		return

	case cfg.MatchStateCountdown:
		updateCountdown(match)

	case cfg.MatchStatePlaying:
		updatePlaying(match)

	case cfg.MatchStateFinished:
		// Results display - timer counts down to return to menu
		if match.Timer > 0 {
			match.Timer--
		}
	}
}

func updateCountdown(match *components.MatchData) {
	if match.Timer > 0 {
		match.Timer--

		// Calculate countdown value (3, 2, 1, 0=GO)
		framesPerCount := cfg.Match.CountdownDuration / 4
		match.CountdownValue = match.Timer / framesPerCount
		if match.CountdownValue > 3 {
			match.CountdownValue = 3
		}
		return
	}

	// Countdown finished - start playing
	match.State = cfg.MatchStatePlaying
	match.Timer = match.Duration
	match.CountdownValue = -1 // -1 means "GO" or no countdown
}

func updatePlaying(match *components.MatchData) {
	if match.Timer > 0 {
		match.Timer--
		return
	}

	// Time's up - determine winner
	match.State = cfg.MatchStateFinished
	match.Timer = cfg.Match.ResultsDisplayTime

	determineWinner(match)
}

func determineWinner(match *components.MatchData) {
	switch match.GameMode {
	case cfg.GameModeFreeForAll, cfg.GameMode1v1:
		match.WinnerIndex = match.GetLeader()
		match.WinningTeam = -1

	case cfg.GameMode2v2:
		team0Score := match.GetTeamScore(0)
		team1Score := match.GetTeamScore(1)

		if team0Score > team1Score {
			match.WinningTeam = 0
		} else if team1Score > team0Score {
			match.WinningTeam = 1
		} else {
			match.WinningTeam = -1 // Tie
		}
		match.WinnerIndex = -1 // Not applicable for team mode

	case cfg.GameModeCoopVsBots:
		// Co-op doesn't have a traditional winner
		match.WinnerIndex = -1
		match.WinningTeam = -1
	}
}

// StartMatch transitions from waiting to countdown
func StartMatch(e *ecs.ECS) {
	matchEntry, ok := components.Match.First(e.World)
	if !ok {
		return
	}
	match := components.Match.Get(matchEntry)

	if match.State != cfg.MatchStateWaiting {
		return
	}

	match.State = cfg.MatchStateCountdown
	match.Timer = cfg.Match.CountdownDuration
	match.CountdownValue = 3
}

// IsMatchPlaying returns true if the match is in the playing state
func IsMatchPlaying(e *ecs.ECS) bool {
	matchEntry, ok := components.Match.First(e.World)
	if !ok {
		return true // No match component = always playing (backwards compat)
	}
	match := components.Match.Get(matchEntry)
	return match.State == cfg.MatchStatePlaying
}

// IsMatchFinished returns true if the match has ended
func IsMatchFinished(e *ecs.ECS) bool {
	matchEntry, ok := components.Match.First(e.World)
	if !ok {
		return false
	}
	match := components.Match.Get(matchEntry)
	return match.State == cfg.MatchStateFinished && match.Timer <= 0
}

// GetMatchTimeRemaining returns the remaining match time in seconds
func GetMatchTimeRemaining(e *ecs.ECS) int {
	matchEntry, ok := components.Match.First(e.World)
	if !ok {
		return 0
	}
	match := components.Match.Get(matchEntry)
	if match.State != cfg.MatchStatePlaying {
		return 0
	}
	return match.Timer / 60 // Convert frames to seconds
}
