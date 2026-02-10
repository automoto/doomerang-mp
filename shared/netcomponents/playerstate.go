package netcomponents

import (
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/yohamta/donburi"
)

type NetPlayerStateData struct {
	StateID      netconfig.StateID
	Direction    int    // -1 left, 1 right
	Health       int
	LastSequence uint32 // Last input sequence processed by the server (for prediction reconciliation)
	IsLocal      bool   // Client-side only, not synced
}

var NetPlayerState = donburi.NewComponentType[NetPlayerStateData]()
