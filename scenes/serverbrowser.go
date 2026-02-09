package scenes

import (
	"image/color"
	"sync"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/network"
	"github.com/automoto/doomerang-mp/systems"
	"github.com/automoto/doomerang-mp/ui"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

type ServerBrowserScene struct {
	ecsWorld     *ecs.ECS
	sceneChanger SceneChanger
	browserUI    *ui.ServerBrowserUI
	netClient    *network.Client
	once         sync.Once
	shouldGoBack bool
}

func NewServerBrowserScene(sc SceneChanger) *ServerBrowserScene {
	return &ServerBrowserScene{sceneChanger: sc}
}

func (s *ServerBrowserScene) Update() {
	s.once.Do(s.configure)

	s.ecsWorld.Update()
	s.browserUI.Update()

	if s.shouldGoBack {
		if s.netClient != nil {
			s.netClient.Disconnect()
			s.netClient = nil
		}
		systems.FadeOutMusic(s.ecsWorld)
		s.sceneChanger.ChangeScene(NewMenuScene(s.sceneChanger))
		return
	}

	if s.netClient != nil {
		switch s.netClient.State() {
		case network.StateJoinedGame:
			s.browserUI.SetStatus("Joined! Loading game...")
			client := s.netClient
			s.netClient = nil // Transfer ownership to networked scene
			s.sceneChanger.ChangeScene(NewNetworkedScene(s.sceneChanger, client))
			return

		case network.StateError:
			errMsg := "Connection failed"
			if err := s.netClient.LastError(); err != nil {
				errMsg = err.Error()
			}
			s.browserUI.SetStatus(errMsg)
			s.browserUI.SetConnecting(false)
			s.netClient.Disconnect()
			s.netClient = nil

		case network.StateConnecting:
			s.browserUI.SetStatus("Connecting...")

		case network.StateConnected:
			s.browserUI.SetStatus("Connected, joining game...")

		case network.StateDisconnected:
			s.browserUI.SetStatus("Disconnected")
			s.browserUI.SetConnecting(false)
			s.netClient = nil
		}
	}
}

func (s *ServerBrowserScene) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 20, 30, 255})

	if s.ecsWorld == nil {
		return
	}

	s.browserUI.UI.Draw(screen)
}

func (s *ServerBrowserScene) configure() {
	s.ecsWorld = ecs.NewECS(donburi.NewWorld())

	s.ecsWorld.AddSystem(systems.UpdateAudio)
	s.browserUI = ui.NewServerBrowserUI(
		func(address string) { s.onConnect(address) },
		func() { s.shouldGoBack = true },
	)

	systems.PlayMusic(s.ecsWorld, cfg.Sound.MenuMusic)
}

func (s *ServerBrowserScene) onConnect(address string) {
	if s.netClient != nil {
		s.netClient.Disconnect()
	}

	s.browserUI.SetStatus("Connecting...")
	s.browserUI.SetConnecting(true)

	s.netClient = network.NewClient()
	s.netClient.Connect(address, cfg.Network.GameVersion, "Player")
}
