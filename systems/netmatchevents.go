package systems

import (
	"context"
	"log"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/network"
	"github.com/yohamta/donburi/ecs"
)

// NewNetMatchEventSystem returns an ECS system that drains match events
// from the network client and triggers UI effects or SFX.
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
				submitGgscaleScore(client, evt.Scores)
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

// submitGgscaleScore fires the local player's KO count off to ggscale
// at match-end. Runs in a goroutine so the ECS update never blocks on
// a network call; errors are logged. No-op when ggscale isn't
// configured (see network.SetSharedGgscale).
func submitGgscaleScore(client *network.Client, scores map[uint32]int) {
	score := int64(scores[uint32(client.NetworkID())]) //nolint:gosec // NetworkId fits in uint32 for the foreseeable player counts
	go func() {
		if err := client.SubmitMyScore(context.Background(), score); err != nil {
			log.Printf("[ggscale] submit failed: %v", err)
			return
		}
		// Successful no-op when ggscale isn't configured — only log
		// when an actual submission happened.
		if gg, _ := network.SharedGgscale(); gg != nil {
			log.Printf("[ggscale] submitted score=%d", score)
		}
	}()
}
