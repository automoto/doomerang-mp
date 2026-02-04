package protocol

import (
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/leap-fish/necs/esync"
)

// Sync ID constants - ID 1 is reserved by necs for NetworkId
const (
	SyncIDNetPosition    uint = 10
	SyncIDNetVelocity    uint = 11
	SyncIDNetPlayerState uint = 12
	SyncIDNetBoomerang   uint = 13
	SyncIDNetEnemy       uint = 14
	SyncIDNetGameState   uint = 15
)

// Interpolation IDs (uint8 for WithInterpFn)
const (
	InterpIDNetPosition  uint8 = 10
	InterpIDNetVelocity  uint8 = 11
	InterpIDNetBoomerang uint8 = 13
	InterpIDNetEnemy     uint8 = 14
)

// RegisterComponents registers all network components with necs for serialization.
// This must be called by both server and client before any network operations.
func RegisterComponents() error {
	// Register with interpolation for smooth client-side rendering
	if err := esync.RegisterComponent(
		SyncIDNetPosition,
		netcomponents.NetPositionData{},
		netcomponents.NetPosition,
		esync.WithInterpFn(InterpIDNetPosition, netcomponents.LerpNetPosition),
	); err != nil {
		return err
	}

	if err := esync.RegisterComponent(
		SyncIDNetVelocity,
		netcomponents.NetVelocityData{},
		netcomponents.NetVelocity,
		esync.WithInterpFn(InterpIDNetVelocity, netcomponents.LerpNetVelocity),
	); err != nil {
		return err
	}

	// PlayerState: no interpolation (discrete state changes)
	if err := esync.RegisterComponent(
		SyncIDNetPlayerState,
		netcomponents.NetPlayerStateData{},
		netcomponents.NetPlayerState,
	); err != nil {
		return err
	}

	if err := esync.RegisterComponent(
		SyncIDNetBoomerang,
		netcomponents.NetBoomerangData{},
		netcomponents.NetBoomerang,
		esync.WithInterpFn(InterpIDNetBoomerang, netcomponents.LerpNetBoomerang),
	); err != nil {
		return err
	}

	if err := esync.RegisterComponent(
		SyncIDNetEnemy,
		netcomponents.NetEnemyData{},
		netcomponents.NetEnemy,
		esync.WithInterpFn(InterpIDNetEnemy, netcomponents.LerpNetEnemy),
	); err != nil {
		return err
	}

	// GameState: no interpolation (discrete state)
	if err := esync.RegisterComponent(
		SyncIDNetGameState,
		netcomponents.NetGameStateData{},
		netcomponents.NetGameState,
	); err != nil {
		return err
	}

	return nil
}
