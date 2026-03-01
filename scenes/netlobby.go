package scenes

import (
	"image/color"
	"sync"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/network"
	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/automoto/doomerang-mp/systems"
	"github.com/automoto/doomerang-mp/ui"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

type NetLobbyScene struct {
	ecsWorld     *ecs.ECS
	sceneChanger SceneChanger
	netClient    *network.Client
	lobbyUI      *ui.NetLobbyUI
	once         sync.Once
	shouldGoBack bool
}

func NewNetLobbyScene(sc SceneChanger, client *network.Client) *NetLobbyScene {
	return &NetLobbyScene{
		sceneChanger: sc,
		netClient:    client,
	}
}

func (ns *NetLobbyScene) Update() {
	ns.once.Do(ns.configure)

	state := ns.netClient.State()
	if state == network.StateDisconnected || state == network.StateError {
		ns.sceneChanger.ChangeScene(NewServerBrowserScene(ns.sceneChanger))
		return
	}

	// Drain lobby updates
	for _, update := range ns.netClient.DrainLobbyUpdates() {
		ns.lobbyUI.UpdateState(update)
	}

	for _, evt := range ns.netClient.DrainMatchEvents() {
		if evt.Type == "match_start" || evt.Type == "countdown_start" {
			ns.sceneChanger.ChangeScene(NewNetworkedScene(ns.sceneChanger, ns.netClient))
			return
		}
	}

	ns.ecsWorld.Update()
	ns.lobbyUI.Update()

	if ns.shouldGoBack {
		ns.netClient.Disconnect()
		ns.sceneChanger.ChangeScene(NewServerBrowserScene(ns.sceneChanger))
		return
	}
}

func (ns *NetLobbyScene) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 20, 30, 255})
	if ns.ecsWorld == nil {
		return
	}
	ns.lobbyUI.UI.Draw(screen)
}

func (ns *NetLobbyScene) configure() {
	ns.ecsWorld = ecs.NewECS(donburi.NewWorld())
	ns.ecsWorld.AddSystem(systems.UpdateAudio)

	localNetID := uint32(ns.netClient.NetworkID())
	levelNames := ns.netClient.LevelNames()

	ns.lobbyUI = ui.NewNetLobbyUI(
		localNetID,
		levelNames,
		func(action messages.LobbyAction) {
			_ = ns.netClient.SendMessage(action)
		},
		func() { ns.shouldGoBack = true },
	)

	systems.PlayMusic(ns.ecsWorld, cfg.Sound.MenuMusic)
}
