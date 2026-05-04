package network

import (
	"sync"

	ggscale "github.com/automoto/ggscale-go"
)

// Process-global ggscale handle. main.go calls SetSharedGgscale once at
// startup (after authenticating against ggscale-server); every
// subsequent NewClient() picks it up automatically. When unset, the
// network client behaves as if ggscale doesn't exist — SubmitMyScore
// becomes a no-op and the rest of the code is unaffected.
//
// A package-level var is used here (rather than threading the client
// through the Game/SceneChanger/scene-constructor chain) because the
// integration is purely cross-cutting: only NewClient and SubmitMyScore
// touch it, no scene needs to know about ggscale, and the scene
// hierarchy already constructs network.Client several layers down from
// where the configuration lives.
var (
	ggscaleMu           sync.RWMutex
	sharedGgscaleClient *ggscale.Client
	sharedLeaderboardID int64
)

// SetSharedGgscale registers the shared ggscale client + leaderboard ID
// for any subsequently-created network.Client. Pass (nil, 0) to clear.
func SetSharedGgscale(c *ggscale.Client, leaderboardID int64) {
	ggscaleMu.Lock()
	defer ggscaleMu.Unlock()
	sharedGgscaleClient = c
	sharedLeaderboardID = leaderboardID
}

// SharedGgscale returns the registered ggscale client + leaderboard ID
// (or (nil, 0) if SetSharedGgscale was never called).
func SharedGgscale() (*ggscale.Client, int64) {
	ggscaleMu.RLock()
	defer ggscaleMu.RUnlock()
	return sharedGgscaleClient, sharedLeaderboardID
}
