package messages

import "github.com/leap-fish/necs/esync"

// JoinRequest is sent by a client after connecting to request joining the game.
type JoinRequest struct {
	Version        string
	PlayerName     string
	ReconnectToken string
}

// JoinAccepted is sent by the server when a client's join request is accepted.
type JoinAccepted struct {
	NetworkID      esync.NetworkId
	ReconnectToken string
	ServerName     string
	TickRate       int
}

// JoinRejected is sent by the server when a client's join request is rejected.
type JoinRejected struct {
	Reason string
}
