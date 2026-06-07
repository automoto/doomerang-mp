package messages

import "github.com/leap-fish/necs/esync"

// JoinRequest is sent by a client after connecting to request joining the game.
type JoinRequest struct {
	Version        string
	PlayerName     string
	ReconnectToken string
	Level          string // Requested level name (empty = server default)

	// GgscaleSessionToken is the player's ggscale access token (the JWT
	// from /v1/auth/anonymous or any other Authenticator). The
	// dedicated game server submits leaderboard scores on the player's
	// behalf at match end via Leaderboards.SubmitFor — clients can't
	// submit directly because publishable keys are blocked from score
	// writes server-side. Empty when ggscale is not configured.
	GgscaleSessionToken string
}

// JoinAccepted is sent by the server when a client's join request is accepted.
type JoinAccepted struct {
	NetworkID      esync.NetworkId
	ReconnectToken string
	ServerName     string
	TickRate       int
	Level          string   // Active level name
	Levels         []string // All available level names
}

// JoinRejected is sent by the server when a client's join request is rejected.
type JoinRejected struct {
	Reason string
}
