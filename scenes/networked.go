package scenes

import (
	"image/color"
	"log"
	"math"
	"strings"
	"sync"

	"github.com/automoto/doomerang-mp/assets"
	"github.com/automoto/doomerang-mp/components"
	"github.com/automoto/doomerang-mp/network"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
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

	// Build prediction collision space from level tile data
	if levelEntry, ok := components.Level.First(ns.ecsWorld.World); ok {
		levelData := components.Level.Get(levelEntry)
		if levelData.CurrentLevel != nil {
			lvl := levelData.CurrentLevel
			// Use first spawn point as initial position
			spawnX, spawnY := 100.0, 100.0
			if len(lvl.PlayerSpawns) > 0 {
				spawnX = lvl.PlayerSpawns[0].X
				spawnY = lvl.PlayerSpawns[0].Y
			}
			ns.prediction.InitCollision(lvl.SolidTiles, lvl.Width, lvl.Height, spawnX, spawnY)
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
	ns.ecsWorld.AddSystem(systems.NewNetCameraSystem(localNetID))
	ns.ecsWorld.AddSystem(systems.UpdateAudio)
	ns.ecsWorld.AddRenderer(cfg.Default, systems.DrawLevel)
	ns.ecsWorld.AddRenderer(cfg.Default, systems.DrawNetworkedPlayers)
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
		}

		entry := world.Entry(entity)

		if ent.Id == myNetID {
			// Local player — reconcile prediction instead of overwriting
			ns.reconcileLocal(entry, compData)
		} else {
			// Remote players — interpolate position, apply other state directly
			// First pass: extract velocity for extrapolation
			var remoteVel *netcomponents.NetVelocityData
			for _, data := range compData {
				if v, ok := data.(netcomponents.NetVelocityData); ok {
					remoteVel = &v
					break
				}
			}

			for _, data := range compData {
				if v, ok := data.(netcomponents.NetPositionData); ok && entry.HasComponent(components.NetInterp) {
					interp := components.NetInterp.Get(entry)
					if !interp.Initialized {
						// First snapshot — set position directly, no interpolation
						applyComponentToEntry(entry, data)
						interp.PrevX = v.X
						interp.PrevY = v.Y
						interp.TargetX = v.X
						interp.TargetY = v.Y
						interp.T = 1.0
						interp.Initialized = true
					} else {
						// Subsequent snapshots — start interpolation from current position
						pos := netcomponents.NetPosition.Get(entry)
						interp.PrevX = pos.X
						interp.PrevY = pos.Y
						interp.TargetX = v.X
						interp.TargetY = v.Y
						interp.T = 0
					}
					// Store velocity for extrapolation
					if remoteVel != nil {
						interp.VelX = remoteVel.SpeedX
						interp.VelY = remoteVel.SpeedY
					}
				} else {
					applyComponentToEntry(entry, data)
				}
			}
		}
	}

	esync.NetworkEntityQuery.Each(world, func(entry *donburi.Entry) {
		id := esync.GetNetworkId(entry)
		if id == nil {
			return
		}
		if !ns.presentIDs[*id] {
			entry.Remove()
		}
	})
}

// reconcileLocal handles server state for the local player using prediction
// reconciliation. Instead of overwriting position, it compares with the
// predicted position and corrects if needed.
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

	// Apply non-position state (health, stateID, etc.) directly
	if serverState != nil {
		if !entry.HasComponent(netcomponents.NetPlayerState) {
			entry.AddComponent(netcomponents.NetPlayerState)
		}
		localState := netcomponents.NetPlayerState.Get(entry)
		localState.StateID = serverState.StateID
		localState.Health = serverState.Health
		localState.IsLocal = true
		// Don't overwrite Direction — local prediction handles it
	}

	// If we have both position and velocity from server, reconcile
	if serverPos != nil && serverVel != nil && serverState != nil {
		if !entry.HasComponent(netcomponents.NetPosition) {
			entry.AddComponent(netcomponents.NetPosition)
		}
		if !entry.HasComponent(netcomponents.NetVelocity) {
			entry.AddComponent(netcomponents.NetVelocity)
		}

		localPos := netcomponents.NetPosition.Get(entry)

		if serverState.LastSequence == 0 || ns.prediction.Buffer.NextSeq() == 0 {
			// No input sent yet — accept server position directly (initial spawn)
			localPos.X = serverPos.X
			localPos.Y = serverPos.Y
			ns.prediction.OnGround = math.Abs(serverVel.SpeedY) < 0.1
			if ns.prediction.PlayerObj != nil {
				ns.prediction.PlayerObj.X = serverPos.X
				ns.prediction.PlayerObj.Y = serverPos.Y
				ns.prediction.PlayerObj.Update()
			}
		} else {
			reconPos := &netcomponents.NetPositionData{X: serverPos.X, Y: serverPos.Y}
			corrected := ns.prediction.Reconcile(reconPos, serverVel, serverState.LastSequence)
			if corrected {
				localPos.X = reconPos.X
				localPos.Y = reconPos.Y
				if ns.prediction.PlayerObj != nil {
					ns.prediction.PlayerObj.X = reconPos.X
					ns.prediction.PlayerObj.Y = reconPos.Y
					ns.prediction.PlayerObj.Update()
				}
			}
		}

		// Always update velocity component with server's authoritative velocity
		localVel := netcomponents.NetVelocity.Get(entry)
		localVel.SpeedX = serverVel.SpeedX
		localVel.SpeedY = serverVel.SpeedY
	} else {
		// Missing components — fallback to direct apply
		for _, data := range components {
			applyComponentToEntry(entry, data)
		}
		if entry.HasComponent(netcomponents.NetPlayerState) {
			netcomponents.NetPlayerState.Get(entry).IsLocal = true
		}
	}
}

// initNetPlayerAnimation attaches Animation and NetInterp components to a networked player entity.
func initNetPlayerAnimation(entry *donburi.Entry) {
	animData := factory.GenerateAnimations("player", cfg.Player.FrameWidth, cfg.Player.FrameHeight)
	animData.SetAnimation(cfg.Idle)
	entry.AddComponent(components.Animation)
	components.Animation.SetValue(entry, *animData)

	entry.AddComponent(components.NetInterp)
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
	}
}
