package messages

import "github.com/automoto/doomerang-mp/config"

// PlayerInput is sent from client to server each frame with the player's input state.
// Used for server-side movement processing and client-side prediction reconciliation.
type PlayerInput struct {
	Sequence  uint32                   // Incrementing ID for reconciliation
	Actions   map[config.ActionID]bool // Which actions are currently pressed
	Direction int                      // -1 left, 0 none, 1 right
	Timestamp int64                    // Client timestamp (Unix ms)
}

// NewPlayerInput creates a PlayerInput with initialized map
func NewPlayerInput(seq uint32) PlayerInput {
	return PlayerInput{
		Sequence: seq,
		Actions:  make(map[config.ActionID]bool),
	}
}
