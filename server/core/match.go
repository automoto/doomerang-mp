package core

import (
	"log"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/leap-fish/necs/esync"
	"github.com/leap-fish/necs/esync/srvsync"
	"github.com/yohamta/donburi"
)

type ServerMatch struct {
	server *Server

	State         netcomponents.MatchStateID
	Timer         float64
	Duration      float64
	CountdownTime float64

	Scores map[uint32]int
	Deaths map[uint32]int

	MinPlayers int
	MaxPlayers int

	GameMode     string
	MatchMinutes int
	LevelIndex   int
	WinnerID     uint32
	HostID       uint32
	Slots        [4]messages.LobbySlot

	// Round system
	CurrentRound int
	RoundWins    map[int]int     // team number -> rounds won
	Lives        map[uint32]int  // NetworkId -> remaining lives
	Eliminated   map[uint32]bool // NetworkId -> eliminated this round

	// Singleton entity in the server world to sync state
	gameStateEntity donburi.Entity
}

func NewServerMatch(server *Server) *ServerMatch {
	m := &ServerMatch{
		server:        server,
		State:         netcomponents.MatchStateWaiting,
		Duration:      float64(cfg.Match.RoundDuration) / 60.0,
		CountdownTime: 3.0,
		Scores:        make(map[uint32]int),
		Deaths:        make(map[uint32]int),
		MinPlayers:    2,
		MaxPlayers:    4,
		GameMode:      "ffa",
		MatchMinutes:  2,
		RoundWins:     make(map[int]int),
		Lives:         make(map[uint32]int),
		Eliminated:    make(map[uint32]bool),
	}

	for i := range m.Slots {
		m.Slots[i].Type = 0
	}

	m.createGameStateEntity()
	return m
}

func (m *ServerMatch) createGameStateEntity() {
	m.gameStateEntity = m.server.world.Create(netcomponents.NetGameState)
	entry := m.server.world.Entry(m.gameStateEntity)
	netcomponents.NetGameState.Set(entry, &netcomponents.NetGameStateData{
		MatchState: m.State,
		GameMode:   m.GameMode,
	})

	err := srvsync.NetworkSync(m.server.world, &m.gameStateEntity, netcomponents.NetGameState)
	if err != nil {
		log.Printf("Failed to setup network sync for game state: %v", err)
	}
}

func (m *ServerMatch) Update(dt float64) {
	switch m.State {
	case netcomponents.MatchStateWaiting:
		m.updateWaiting()
	case netcomponents.MatchStateCountdown:
		m.updateCountdown(dt)
	case netcomponents.MatchStatePlaying:
		m.updatePlaying(dt)
	case netcomponents.MatchStateRoundEnd:
		m.updateRoundEnd(dt)
	case netcomponents.MatchStateFinished:
		m.updateFinished(dt)
	}

	m.syncGameState()
}

func (m *ServerMatch) updateWaiting() {
	// Manual start from lobby only
}

func (m *ServerMatch) startCountdown() {
	m.State = netcomponents.MatchStateCountdown
	m.Timer = m.CountdownTime

	m.server.broadcastEvent(messages.MatchEvent{
		Type:    "countdown_start",
		Message: "Match starting...",
	})
}

func (m *ServerMatch) updateCountdown(dt float64) {
	m.Timer -= dt

	if m.Timer <= 0 {
		m.startMatch()
	}
}

func (m *ServerMatch) startMatch() {
	m.State = netcomponents.MatchStatePlaying
	m.server.matchInProgress.Store(true)
	m.Timer = m.Duration

	m.Scores = make(map[uint32]int)
	m.Deaths = make(map[uint32]int)
	m.WinnerID = 0
	m.CurrentRound = 1
	m.RoundWins = make(map[int]int)

	// Clear all existing player entities
	m.server.ClearAllPlayers()

	// Spawn players from lobby slots
	for i, slot := range m.Slots {
		if slot.Type != 0 {
			m.server.SpawnPlayerAtSlot(i, slot)
		}
	}

	m.initLivesForAllPlayers()

	m.server.broadcastEvent(messages.MatchEvent{
		Type:    "match_start",
		Message: "FIGHT!",
	})
}

func (m *ServerMatch) updatePlaying(dt float64) {
	m.Timer -= dt

	if m.Timer <= 0 {
		m.handleTimerExpiry()
		return
	}
}

func (m *ServerMatch) handleTimerExpiry() {
	winnerTeam := m.determineRoundWinner()
	m.endRound(winnerTeam)
}

func (m *ServerMatch) endRound(winnerTeam int) {
	m.State = netcomponents.MatchStateRoundEnd
	m.Timer = float64(cfg.Match.RoundEndDelay) / 60.0

	if winnerTeam >= 0 {
		m.RoundWins[winnerTeam]++
	}

	// Find a winner network ID for display
	winnerNetID := m.netIDForTeam(winnerTeam)

	m.server.broadcastEvent(messages.MatchEvent{
		Type:        "round_end",
		WinnerID:    winnerNetID,
		RoundNumber: m.CurrentRound,
		Message:     "Round over!",
	})
}

func (m *ServerMatch) updateRoundEnd(dt float64) {
	m.Timer -= dt
	if m.Timer > 0 {
		return
	}

	// Check if someone has won enough rounds
	for team, wins := range m.RoundWins {
		if wins >= cfg.Match.RoundsToWin {
			m.WinnerID = m.netIDForTeam(team)
			m.endMatch("rounds")
			return
		}
	}

	m.startNextRound()
}

func (m *ServerMatch) startNextRound() {
	m.CurrentRound++

	// Clear and respawn all players
	m.server.ClearAllPlayers()
	for i, slot := range m.Slots {
		if slot.Type != 0 {
			m.server.SpawnPlayerAtSlot(i, slot)
		}
	}

	m.initLivesForAllPlayers()

	// Reset round timer
	m.Timer = m.Duration

	// Go through countdown
	m.State = netcomponents.MatchStateCountdown
	m.Timer = m.CountdownTime

	m.server.broadcastEvent(messages.MatchEvent{
		Type:    "countdown_start",
		Message: "Next round...",
	})
}

func (m *ServerMatch) endMatch(reason string) {
	m.State = netcomponents.MatchStateFinished
	m.server.matchInProgress.Store(false)

	// If winner not already set, determine it
	if m.WinnerID == 0 {
		m.WinnerID = m.determineWinner()
	}

	m.server.broadcastEvent(messages.MatchEvent{
		Type:     "match_end",
		WinnerID: m.WinnerID,
		Reason:   reason,
		Scores:   m.Scores,
	})

	// Server-authoritative leaderboard submission: hand the final
	// scores + per-player session tokens to the configured hook,
	// which calls Leaderboards.SubmitFor. Runs in a goroutine because
	// the hook makes network calls and must not block the game loop.
	scoresCopy := make(map[uint32]int, len(m.Scores))
	for k, v := range m.Scores {
		scoresCopy[k] = v
	}
	go m.server.invokeMatchEndHook(scoresCopy)

	m.Timer = 10.0
}

func (m *ServerMatch) determineWinner() uint32 {
	var winnerID uint32
	maxKOs := -1

	for netID, kos := range m.Scores {
		if kos > maxKOs {
			maxKOs = kos
			winnerID = netID
		}
	}

	return winnerID
}

// determineRoundWinner picks the winning team when the timer expires.
// Tiebreaker: most lives remaining -> most KOs -> first found.
func (m *ServerMatch) determineRoundWinner() int {
	teamLives := make(map[int]int)
	teamKOs := make(map[int]int)

	for i, slot := range m.Slots {
		if slot.Type == 0 {
			continue
		}
		team := m.getPlayerTeam(i)
		nid := m.slotNetID(i)
		if nid == 0 {
			continue
		}
		teamLives[team] += m.Lives[nid]
		teamKOs[team] += m.Scores[nid]
	}

	bestTeam := -1
	bestLives := -1
	bestKOs := -1

	for team, lives := range teamLives {
		kos := teamKOs[team]
		if lives > bestLives || (lives == bestLives && kos > bestKOs) {
			bestTeam = team
			bestLives = lives
			bestKOs = kos
		}
	}

	return bestTeam
}

// checkRoundEndCondition checks if only one team has alive players.
func (m *ServerMatch) checkRoundEndCondition() {
	if m.State != netcomponents.MatchStatePlaying {
		return
	}

	teamsAlive := make(map[int]bool)
	for i, slot := range m.Slots {
		if slot.Type == 0 {
			continue
		}
		nid := m.slotNetID(i)
		if nid == 0 {
			continue
		}
		if !m.Eliminated[nid] {
			teamsAlive[m.getPlayerTeam(i)] = true
		}
	}

	switch len(teamsAlive) {
	case 0:
		// Mutual kill — restart round without awarding a win
		m.endRound(-1) // -1 means no winner
	case 1:
		// One team left standing
		for team := range teamsAlive {
			m.endRound(team)
		}
	}
}

func (m *ServerMatch) getPlayerTeam(slotIdx int) int {
	// In FFA mode, each slot is its own team
	if m.GameMode == "ffa" || m.GameMode == "1v1" {
		return slotIdx
	}
	// Team modes use the slot's Team field
	return m.Slots[slotIdx].Team
}

func (m *ServerMatch) initLivesForAllPlayers() {
	m.Lives = make(map[uint32]int)
	m.Eliminated = make(map[uint32]bool)

	for entity := range m.server.playerPhysics {
		if !m.server.world.Valid(entity) {
			continue
		}
		entry := m.server.world.Entry(entity)
		nid := esync.GetNetworkId(entry)
		if nid != nil {
			m.Lives[uint32(*nid)] = cfg.Match.LivesPerRound
		}
	}
}

// netIDForTeam returns the first network ID found for the given team (for display purposes).
func (m *ServerMatch) netIDForTeam(team int) uint32 {
	for i, slot := range m.Slots {
		if slot.Type == 0 {
			continue
		}
		if m.getPlayerTeam(i) == team {
			nid := m.slotNetID(i)
			if nid != 0 {
				return nid
			}
		}
	}
	return 0
}

// slotNetID returns the network ID for a given slot. For humans it comes from
// Slots[].PlayerID. For bots we look it up from playerPhysics.
func (m *ServerMatch) slotNetID(slotIdx int) uint32 {
	slot := m.Slots[slotIdx]
	if slot.Type == 1 {
		return slot.PlayerID
	}
	// Bot — find entity with matching PlayerIndex
	for entity := range m.server.playerPhysics {
		if !m.server.world.Valid(entity) {
			continue
		}
		entry := m.server.world.Entry(entity)
		nid := esync.GetNetworkId(entry)
		if nid == nil {
			continue
		}
		// Check if this is the bot for this slot by checking stored SlotNetIDs
		// We can also check by iterating slots and matching
		if m.Slots[slotIdx].PlayerID == uint32(*nid) {
			return uint32(*nid)
		}
	}
	return 0
}

func (m *ServerMatch) updateFinished(dt float64) {
	m.Timer -= dt

	if m.Timer <= 0 {
		if m.server.PlayerCount() >= m.MinPlayers {
			m.startCountdown()
		} else {
			m.State = netcomponents.MatchStateWaiting
		}
	}
}

func (m *ServerMatch) syncGameState() {
	entry := m.server.world.Entry(m.gameStateEntity)
	state := netcomponents.NetGameState.Get(entry)

	state.MatchState = m.State
	state.TimeRemaining = m.Timer
	state.Scores = m.Scores
	state.Deaths = m.Deaths
	state.WinnerID = m.WinnerID
	state.GameMode = m.GameMode

	// Round system
	state.CurrentRound = m.CurrentRound
	state.RoundWins = m.RoundWins
	state.Lives = m.Lives
	state.Eliminated = m.Eliminated
	state.RoundsToWin = cfg.Match.RoundsToWin

	// Slot info for HUD
	for i, slot := range m.Slots {
		state.SlotNetIDs[i] = m.slotNetID(i)
		state.SlotNames[i] = slot.Name
		state.SlotTypes[i] = slot.Type
		state.SlotTeams[i] = m.getPlayerTeam(i)
	}
}

func (m *ServerMatch) AddKO(killerID uint32) {
	m.Scores[killerID]++
	m.server.broadcastEvent(messages.ScoreEvent{
		PlayerID: killerID,
		KOs:      m.Scores[killerID],
		Deaths:   m.Deaths[killerID],
	})
}

func (m *ServerMatch) AddDeath(victimID uint32) {
	m.Deaths[victimID]++
	m.server.broadcastEvent(messages.ScoreEvent{
		PlayerID: victimID,
		KOs:      m.Scores[victimID],
		Deaths:   m.Deaths[victimID],
	})
}

func (m *ServerMatch) OnLobbyAction(playerID uint32, action messages.LobbyAction) {
	if m.State != netcomponents.MatchStateWaiting {
		return
	}

	// Update HostID if not set
	if m.HostID == 0 {
		m.HostID = playerID
	}

	switch action.Action {
	case "pick_slot":
		slotIdx := action.Value
		if slotIdx < 0 || slotIdx >= 4 {
			return
		}
		// Clear player from previous slot
		for i := range m.Slots {
			if m.Slots[i].PlayerID == playerID {
				m.Slots[i].Type = 0
				m.Slots[i].PlayerID = 0
				m.Slots[i].Ready = false
			}
		}
		// Try to pick new slot
		if m.Slots[slotIdx].Type == 0 {
			m.Slots[slotIdx].Type = 1 // Human
			m.Slots[slotIdx].PlayerID = playerID
			m.Slots[slotIdx].Ready = false
			m.Slots[slotIdx].Name = action.String
		}

	case "ready":
		for i := range m.Slots {
			if m.Slots[i].PlayerID == playerID {
				m.Slots[i].Ready = true
			}
		}

	case "unready":
		for i := range m.Slots {
			if m.Slots[i].PlayerID == playerID {
				m.Slots[i].Ready = false
			}
		}

	case "change_mode":
		if playerID == m.HostID {
			m.GameMode = action.String
		}

	case "change_time":
		if playerID == m.HostID {
			m.MatchMinutes = action.Value
			m.Duration = float64(m.MatchMinutes * 60)
		}

	case "change_level":
		if playerID == m.HostID {
			m.LevelIndex = action.Value
		}

	case "add_bot":
		if playerID == m.HostID {
			for i := range m.Slots {
				if m.Slots[i].Type == 0 {
					m.Slots[i].Type = 2 // Bot
					m.Slots[i].Difficulty = action.Value
					m.Slots[i].Name = "Bot"
					break
				}
			}
		}

	case "remove_bot":
		if playerID == m.HostID {
			slotIdx := action.Value
			if slotIdx >= 0 && slotIdx < 4 && m.Slots[slotIdx].Type == 2 {
				m.Slots[slotIdx].Type = 0
				m.Slots[slotIdx].Difficulty = 0
				m.Slots[slotIdx].Name = ""
			}
		}

	case "start_match":
		if playerID == m.HostID && m.canStart() {
			m.startCountdown()
		}
	}

	m.broadcastLobbyUpdate()

	// Check if all ready to start
	if m.canStart() {
		m.startCountdown()
	}
}

func (m *ServerMatch) broadcastLobbyUpdate() {
	m.server.broadcastEvent(messages.LobbyUpdate{
		Slots:        m.Slots,
		GameMode:     m.GameMode,
		MatchMinutes: m.MatchMinutes,
		LevelIndex:   m.LevelIndex,
		HostID:       m.HostID,
	})
}

func (m *ServerMatch) canStart() bool {
	if m.State != netcomponents.MatchStateWaiting {
		return false
	}

	humanCount := 0
	totalCount := 0
	readyCount := 0
	for _, slot := range m.Slots {
		if slot.Type == 1 { // Human
			humanCount++
			totalCount++
			if slot.Ready {
				readyCount++
			}
		} else if slot.Type == 2 { // Bot
			totalCount++
		}
	}
	return humanCount > 0 && humanCount == readyCount && totalCount >= m.MinPlayers
}

func (m *ServerMatch) OnDisconnect(playerID uint32) {
	// Remove player from slot
	for i := range m.Slots {
		if m.Slots[i].PlayerID == playerID {
			m.Slots[i].Type = 0
			m.Slots[i].PlayerID = 0
			m.Slots[i].Ready = false
			m.Slots[i].Name = ""
		}
	}

	// If match is active, mark the disconnected player as eliminated
	if m.State == netcomponents.MatchStatePlaying || m.State == netcomponents.MatchStateRoundEnd {
		m.Eliminated[playerID] = true
		m.Lives[playerID] = 0
		if m.State == netcomponents.MatchStatePlaying {
			m.checkRoundEndCondition()
		}
	}

	// Reassign host if needed
	if m.HostID == playerID {
		m.HostID = 0
		for _, slot := range m.Slots {
			if slot.Type == 1 { // Human
				m.HostID = slot.PlayerID
				break
			}
		}
	}

	m.broadcastLobbyUpdate()
}

func (m *ServerMatch) FirstEmptySlot() int {
	for i := range m.Slots {
		if m.Slots[i].Type == 0 {
			return i
		}
	}
	return -1
}
