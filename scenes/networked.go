package scenes

import (
	"image/color"
	"log"
	"math"
	"strings"
	"sync"

	"github.com/automoto/doomerang-mp/assets"
	"github.com/automoto/doomerang-mp/components"
	"github.com/automoto/doomerang-mp/mathutil"
	"github.com/automoto/doomerang-mp/network"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/shared/netconfig"
	"github.com/automoto/doomerang-mp/systems"
	"github.com/automoto/doomerang-mp/systems/factory"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/leap-fish/necs/esync"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"

	cfg "github.com/automoto/doomerang-mp/config"
)

type NetworkedScene struct {
	ecsWorld     *ecs.ECS
	sceneChanger SceneChanger
	netClient    *network.Client
	prediction   *systems.NetPrediction
	once         sync.Once
	presentIDs   map[esync.NetworkId]bool
}

func NewNetworkedScene(sc SceneChanger, client *network.Client) *NetworkedScene {
	return &NetworkedScene{
		sceneChanger: sc,
		netClient:    client,
		prediction:   systems.NewNetPrediction(),
		presentIDs:   make(map[esync.NetworkId]bool),
	}
}

func (ns *NetworkedScene) Update() {
	ns.once.Do(ns.configure)

	state := ns.netClient.State()
	if state == network.StateDisconnected || state == network.StateError {
		log.Println("[networked] disconnected, returning to browser")
		ns.netClient.Disconnect()
		ns.sceneChanger.ChangeScene(NewServerBrowserScene(ns.sceneChanger))
		return
	}

	if snap := ns.netClient.LatestSnapshot(); snap != nil {
		ns.applySnapshot(*snap)
	}

	ns.ecsWorld.Update()
}

func (ns *NetworkedScene) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)

	if ns.ecsWorld == nil {
		return
	}

	ns.ecsWorld.Draw(screen)
}

func (ns *NetworkedScene) configure() {
	// Preload sprite sheets and shaders for animated rendering
	assets.PreloadAllAnimations()
	if err := assets.LoadShaders(); err != nil {
		log.Println("[networked] failed to load shaders:", err)
	}

	ns.ecsWorld = ecs.NewECS(donburi.NewWorld())

	// Load level matching the server's active level
	levelIndex := findLevelIndex(ns.netClient.Level())
	factory.CreateLevelAtIndex(ns.ecsWorld, levelIndex)
	factory.CreateCamera(ns.ecsWorld)

	// Build prediction collision space + VFX space from level data
	if levelEntry, ok := components.Level.First(ns.ecsWorld.World); ok {
		levelData := components.Level.Get(levelEntry)
		if levelData.CurrentLevel != nil {
			lvl := levelData.CurrentLevel
			spawnX, spawnY := 100.0, 100.0
			if len(lvl.PlayerSpawns) > 0 {
				spawnX = lvl.PlayerSpawns[0].X
				spawnY = lvl.PlayerSpawns[0].Y
			}
			ns.prediction.InitCollision(lvl.SolidTiles, lvl.Width, lvl.Height, spawnX, spawnY)
			factory.CreateSpace(ns.ecsWorld, lvl.Width, lvl.Height, 16, 16)
		}
	}

	sendFn := func(msg any) error {
		if ns.netClient.State() != network.StateJoinedGame {
			return nil
		}
		return ns.netClient.SendMessage(msg)
	}
	localNetID := func() esync.NetworkId {
		return ns.netClient.NetworkID()
	}
	ns.ecsWorld.AddSystem(systems.NewNetworkInputSystem(sendFn, ns.prediction, localNetID))
	ns.ecsWorld.AddSystem(systems.NewNetInterpSystem(ns.netClient.TickRate))
	ns.ecsWorld.AddSystem(systems.UpdateNetAnimations)
	ns.ecsWorld.AddSystem(systems.NewNetPlayerEffectsSystem(ns.prediction, localNetID))
	ns.ecsWorld.AddSystem(systems.NewNetCameraSystem(localNetID))
	ns.ecsWorld.AddSystem(systems.NewNetBoomerangEventSystem(ns.netClient))
	ns.ecsWorld.AddSystem(systems.UpdateEffects)
	ns.ecsWorld.AddSystem(systems.UpdateAudio)
	ns.ecsWorld.AddRenderer(cfg.Default, systems.DrawLevel)
	ns.ecsWorld.AddRenderer(cfg.Default, systems.DrawNetworkedPlayers)
	ns.ecsWorld.AddRenderer(cfg.Default, systems.DrawNetworkedBoomerangs)
	ns.ecsWorld.AddRenderer(cfg.Default, systems.DrawAnimated)
	ns.ecsWorld.AddRenderer(cfg.Default, systems.DrawNetworkHUD)
}

// findLevelIndex returns the index of the level matching name, or 0 if not found.
func findLevelIndex(name string) int {
	names := assets.NewLevelLoader().ListLevelNames()
	for i, n := range names {
		if strings.EqualFold(n, name) {
			return i
		}
	}
	return 0
}

func (ns *NetworkedScene) applySnapshot(snapshot esync.WorldSnapshot) {
	world := ns.ecsWorld.World
	myNetID := ns.netClient.NetworkID()

	clear(ns.presentIDs)

	for _, ent := range snapshot {
		ns.presentIDs[ent.Id] = true

		var compData []any
		for _, componentBytes := range ent.State {
			instance, err := esync.Mapper.Deserialize(componentBytes)
			if err != nil {
				continue
			}
			compData = append(compData, instance)
		}

		entity := esync.FindByNetworkId(world, ent.Id)
		if !world.Valid(entity) {
			ctypes := componentTypesFromInstances(compData)
			entity = world.Create(ctypes...)

			entry := world.Entry(entity)
			entry.AddComponent(esync.NetworkIdComponent)
			esync.NetworkIdComponent.SetValue(entry, ent.Id)

			initNetPlayerAnimation(entry)
			initNetBoomerangSprite(entry)
		}

		entry := world.Entry(entity)

		if ent.Id == myNetID {
			ns.reconcileLocal(entry, compData)
			continue
		}

		// Remote entities — extract velocity for extrapolation
		var remoteVel *netcomponents.NetVelocityData
		for _, data := range compData {
			if v, ok := data.(netcomponents.NetVelocityData); ok {
				remoteVel = &v
				break
			}
		}

		for _, data := range compData {
			switch v := data.(type) {
			case netcomponents.NetPositionData:
				if !entry.HasComponent(components.NetInterp) {
					applyComponentToEntry(entry, data)
					break
				}
				interp := components.NetInterp.Get(entry)
				updateInterp(interp, v.X, v.Y,
					func() { applyComponentToEntry(entry, data) },
					func() (float64, float64) {
						pos := netcomponents.NetPosition.Get(entry)
						return pos.X, pos.Y
					},
				)
				if remoteVel != nil {
					interp.VelX = remoteVel.SpeedX
					interp.VelY = remoteVel.SpeedY
				}

			case netcomponents.NetBoomerangData:
				if !entry.HasComponent(components.NetInterp) {
					applyComponentToEntry(entry, data)
					break
				}
				interp := components.NetInterp.Get(entry)
				updateInterp(interp, v.X, v.Y,
					func() { applyComponentToEntry(entry, data) },
					func() (float64, float64) {
						nb := netcomponents.NetBoomerang.Get(entry)
						return nb.X, nb.Y
					},
				)
				interp.VelX = v.VelX
				interp.VelY = v.VelY

			default:
				applyComponentToEntry(entry, data)
			}
		}
	}

	// Collect stale entities first, then remove in a separate pass.
	// Removing during Each iteration can skip entities due to donburi's
	// swap-remove archetype storage, causing stale entities to accumulate.
	var stale []donburi.Entity
	esync.NetworkEntityQuery.Each(world, func(entry *donburi.Entry) {
		id := esync.GetNetworkId(entry)
		if id == nil {
			return
		}
		if !ns.presentIDs[*id] {
			stale = append(stale, entry.Entity())
		}
	})
	for _, entity := range stale {
		world.Remove(entity)
	}
}

// reconcileLocal handles server state for the local player using position
// smoothing. Instead of seq-based reconciliation + replay, it compares the
// current server position with the current local position and applies a small,
// capped correction per snapshot.
func (ns *NetworkedScene) reconcileLocal(entry *donburi.Entry, components []any) {
	var serverPos *netcomponents.NetPositionData
	var serverVel *netcomponents.NetVelocityData
	var serverState *netcomponents.NetPlayerStateData

	for _, data := range components {
		switch v := data.(type) {
		case netcomponents.NetPositionData:
			cp := v
			serverPos = &cp
		case netcomponents.NetVelocityData:
			cp := v
			serverVel = &cp
		case netcomponents.NetPlayerStateData:
			cp := v
			serverState = &cp
		}
	}

	if serverState != nil {
		if !entry.HasComponent(netcomponents.NetPlayerState) {
			entry.AddComponent(netcomponents.NetPlayerState)
		}
		localState := netcomponents.NetPlayerState.Get(entry)
		localState.Health = serverState.Health
		localState.IsLocal = true

		// Let locked animation states play to completion before accepting server transitions
		locked := localState.StateID == netconfig.Throw || localState.StateID == netconfig.Hit
		if locked && serverState.StateID != localState.StateID && animStillPlaying(entry) {
			// Keep local state — animation still playing
		} else {
			localState.StateID = serverState.StateID
		}
	}

	if serverPos == nil || serverVel == nil {
		// Missing components — fallback to direct apply
		for _, data := range components {
			applyComponentToEntry(entry, data)
		}
		if entry.HasComponent(netcomponents.NetPlayerState) {
			netcomponents.NetPlayerState.Get(entry).IsLocal = true
		}
		return
	}

	if !entry.HasComponent(netcomponents.NetPosition) {
		entry.AddComponent(netcomponents.NetPosition)
	}
	if !entry.HasComponent(netcomponents.NetVelocity) {
		entry.AddComponent(netcomponents.NetVelocity)
	}

	localPos := netcomponents.NetPosition.Get(entry)

	if !ns.prediction.Initialized {
		// First snapshot: snap to server position
		localPos.X = serverPos.X
		localPos.Y = serverPos.Y
		ns.prediction.VelX = serverVel.SpeedX
		ns.prediction.VelY = serverVel.SpeedY
		ns.prediction.OnGround = math.Abs(serverVel.SpeedY) < 0.1
		ns.prediction.Initialized = true
		if ns.prediction.PlayerObj != nil {
			ns.prediction.PlayerObj.X = serverPos.X
			ns.prediction.PlayerObj.Y = serverPos.Y
			ns.prediction.PlayerObj.Update()
		}
	} else {
		// Compare current positions — no seq lookup, no replay
		errX := serverPos.X - localPos.X
		errY := serverPos.Y - localPos.Y
		dist := math.Sqrt(errX*errX + errY*errY)

		if dist > cfg.Netcode.SnapThreshold {
			// Teleport/respawn: hard snap
			localPos.X = serverPos.X
			localPos.Y = serverPos.Y
			ns.prediction.VelX = serverVel.SpeedX
			ns.prediction.VelY = serverVel.SpeedY
		} else if dist > cfg.Netcode.SmoothThreshold {
			// Gentle nudge, capped per tick
			corrX := mathutil.ClampFloat(errX*cfg.Netcode.CorrectionRate, -cfg.Netcode.MaxCorrPerTick, cfg.Netcode.MaxCorrPerTick)
			corrY := mathutil.ClampFloat(errY*cfg.Netcode.CorrectionRate, -cfg.Netcode.MaxCorrPerTick, cfg.Netcode.MaxCorrPerTick)
			localPos.X += corrX
			localPos.Y += corrY
			ns.prediction.VelX += (serverVel.SpeedX - ns.prediction.VelX) * cfg.Netcode.VelocityBlendRate
			ns.prediction.VelY += (serverVel.SpeedY - ns.prediction.VelY) * cfg.Netcode.VelocityBlendRate
		}

		// Sync collision object + ground state
		if ns.prediction.PlayerObj != nil {
			ns.prediction.PlayerObj.X = localPos.X
			ns.prediction.PlayerObj.Y = localPos.Y
			ns.prediction.PlayerObj.Update()
		}
		ns.prediction.OnGround = math.Abs(serverVel.SpeedY) < 0.1
	}

	localVel := netcomponents.NetVelocity.Get(entry)
	localVel.SpeedX = serverVel.SpeedX
	localVel.SpeedY = serverVel.SpeedY
}

// initNetBoomerangSprite attaches Sprite and NetInterp components to a boomerang entity.
func initNetBoomerangSprite(entry *donburi.Entry) {
	if !entry.HasComponent(netcomponents.NetBoomerang) {
		return
	}
	img := assets.GetObjectImage("boom_green.png")
	entry.AddComponent(components.Sprite)
	components.Sprite.SetValue(entry, components.SpriteData{
		Image: img,
	})
	entry.AddComponent(components.NetInterp)
}

// initNetPlayerAnimation attaches Animation and NetInterp components to a networked player entity.
// Skips non-player entities (e.g. boomerangs).
func initNetPlayerAnimation(entry *donburi.Entry) {
	if !entry.HasComponent(netcomponents.NetPosition) {
		return
	}
	animData := factory.GenerateAnimations("player", cfg.Player.FrameWidth, cfg.Player.FrameHeight)
	animData.SetAnimation(cfg.Idle)
	entry.AddComponent(components.Animation)
	components.Animation.SetValue(entry, *animData)

	entry.AddComponent(components.NetInterp)
}

// updateInterp sets interpolation targets. On first snapshot it snaps directly
// and calls initFn to set the underlying component data; on subsequent snapshots
// it reads current position via prevFn and starts interpolation.
func updateInterp(interp *components.NetInterpData, targetX, targetY float64, initFn func(), prevFn func() (float64, float64)) {
	if !interp.Initialized {
		initFn()
		interp.PrevX = targetX
		interp.PrevY = targetY
		interp.TargetX = targetX
		interp.TargetY = targetY
		interp.T = 1.0
		interp.Initialized = true
		return
	}
	prevX, prevY := prevFn()
	interp.PrevX = prevX
	interp.PrevY = prevY
	interp.TargetX = targetX
	interp.TargetY = targetY
	interp.T = 0
}

func animStillPlaying(entry *donburi.Entry) bool {
	if !entry.HasComponent(components.Animation) {
		return false
	}
	animData := components.Animation.Get(entry)
	return animData.CurrentAnimation != nil && !animData.CurrentAnimation.Looped
}

func componentTypesFromInstances(components []any) []donburi.IComponentType {
	var ctypes []donburi.IComponentType
	for _, data := range components {
		switch data.(type) {
		case netcomponents.NetPositionData:
			ctypes = append(ctypes, netcomponents.NetPosition)
		case netcomponents.NetVelocityData:
			ctypes = append(ctypes, netcomponents.NetVelocity)
		case netcomponents.NetPlayerStateData:
			ctypes = append(ctypes, netcomponents.NetPlayerState)
		case netcomponents.NetBoomerangData:
			ctypes = append(ctypes, netcomponents.NetBoomerang)
		}
	}
	return ctypes
}

func applyComponentToEntry(entry *donburi.Entry, data any) {
	switch v := data.(type) {
	case netcomponents.NetPositionData:
		if !entry.HasComponent(netcomponents.NetPosition) {
			entry.AddComponent(netcomponents.NetPosition)
		}
		netcomponents.NetPosition.SetValue(entry, v)
	case netcomponents.NetVelocityData:
		if !entry.HasComponent(netcomponents.NetVelocity) {
			entry.AddComponent(netcomponents.NetVelocity)
		}
		netcomponents.NetVelocity.SetValue(entry, v)
	case netcomponents.NetPlayerStateData:
		if !entry.HasComponent(netcomponents.NetPlayerState) {
			entry.AddComponent(netcomponents.NetPlayerState)
		}
		netcomponents.NetPlayerState.SetValue(entry, v)
	case netcomponents.NetBoomerangData:
		if !entry.HasComponent(netcomponents.NetBoomerang) {
			entry.AddComponent(netcomponents.NetBoomerang)
		}
		netcomponents.NetBoomerang.SetValue(entry, v)
	}
}
