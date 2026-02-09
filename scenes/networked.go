package scenes

import (
	"image/color"
	"log"
	"sync"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/network"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/automoto/doomerang-mp/systems"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/leap-fish/necs/esync"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

type NetworkedScene struct {
	ecsWorld     *ecs.ECS
	sceneChanger SceneChanger
	netClient    *network.Client
	once         sync.Once
	presentIDs   map[esync.NetworkId]bool
}

func NewNetworkedScene(sc SceneChanger, client *network.Client) *NetworkedScene {
	return &NetworkedScene{
		sceneChanger: sc,
		netClient:    client,
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
	ns.ecsWorld = ecs.NewECS(donburi.NewWorld())

	sendFn := func(msg any) error {
		if ns.netClient.State() != network.StateJoinedGame {
			return nil
		}
		return ns.netClient.SendMessage(msg)
	}
	ns.ecsWorld.AddSystem(systems.NewNetworkInputSystem(sendFn))
	ns.ecsWorld.AddSystem(systems.UpdateAudio)
	ns.ecsWorld.AddRenderer(cfg.Default, systems.DrawNetworkedPlayers)
	ns.ecsWorld.AddRenderer(cfg.Default, systems.DrawNetworkHUD)
}

func (ns *NetworkedScene) applySnapshot(snapshot esync.WorldSnapshot) {
	world := ns.ecsWorld.World
	myNetID := ns.netClient.NetworkID()

	clear(ns.presentIDs)

	for _, ent := range snapshot {
		ns.presentIDs[ent.Id] = true

		var components []any
		for _, componentBytes := range ent.State {
			instance, err := esync.Mapper.Deserialize(componentBytes)
			if err != nil {
				continue
			}
			components = append(components, instance)
		}

		entity := esync.FindByNetworkId(world, ent.Id)
		if !world.Valid(entity) {
			ctypes := componentTypesFromInstances(components)
			entity = world.Create(ctypes...)

			entry := world.Entry(entity)
			entry.AddComponent(esync.NetworkIdComponent)
			esync.NetworkIdComponent.SetValue(entry, ent.Id)
		}

		entry := world.Entry(entity)
		for _, data := range components {
			applyComponentToEntry(entry, data)
		}

		if ent.Id == myNetID && entry.HasComponent(netcomponents.NetPlayerState) {
			netcomponents.NetPlayerState.Get(entry).IsLocal = true
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
