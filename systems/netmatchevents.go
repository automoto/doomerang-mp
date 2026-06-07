package systems

import (
	"log"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/network"
	"github.com/yohamta/donburi/ecs"
)

// NewNetMatchEventSystem returns an ECS system that drains match events
// from the network client and triggers UI effects or SFX. Leaderboard
// score submission is server-authoritative — the dedicated game server
// submits via Leaderboards.SubmitFor at match end using its secret-tier
// API key, so this system has no leaderboard responsibilities.
func NewNetMatchEventSystem(client *network.Client) func(*ecs.ECS) {
	return func(e *ecs.ECS) {
		for _, evt := range client.DrainMatchEvents() {
			log.Printf("[match-event] Type: %s, Message: %s", evt.Type, evt.Message)

			switch evt.Type {
			case "countdown_start":
				// Could play a whoosh or alert SFX
			case "match_start":
				PlaySFX(e, cfg.SoundBoomerangCatch) // TODO: Better sound for "GO!"
			case "match_end":
				PlaySFX(e, cfg.SoundBoomerangImpact) // TODO: Better sound for match end
			case "round_end":
				PlaySFX(e, cfg.SoundBoomerangImpact) // TODO: Better sound for round end
			case "player_eliminated":
				PlaySFX(e, cfg.SoundBoomerangImpact) // TODO: Elimination sound
			}
		}

		for _, evt := range client.DrainScoreEvents() {
			log.Printf("[score-event] PlayerID: %d, KOs: %d, Deaths: %d", evt.PlayerID, evt.KOs, evt.Deaths)
		}
	}
}
