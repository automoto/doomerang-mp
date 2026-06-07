package scenes

import (
	"context"
	"image/color"
	"log"
	"sync"
	"time"

	"github.com/automoto/doomerang-mp/assets"
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

	mu             sync.Mutex
	fetchedServers []ui.ServerEntry
	fetchErr       error
	fetchDone      bool
}

func NewServerBrowserScene(sc SceneChanger) *ServerBrowserScene {
	return &ServerBrowserScene{
		sceneChanger: sc,
	}
}

func (s *ServerBrowserScene) Update() {
	s.once.Do(s.configure)

	s.ecsWorld.Update()
	s.browserUI.Update()

	// Apply fetch results on the main goroutine
	s.mu.Lock()
	if s.fetchDone {
		servers := s.fetchedServers
		err := s.fetchErr
		s.fetchDone = false
		s.fetchedServers = nil
		s.fetchErr = nil
		s.mu.Unlock()

		s.browserUI.SetRefreshing(false)
		if err != nil {
			s.browserUI.SetBrowseStatus(err.Error())
		} else {
			s.browserUI.SetServerList(servers)
			s.browserUI.SetBrowseStatus("")
		}
	} else {
		s.mu.Unlock()
	}

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
			s.browserUI.SetStatus("Joined! Entering lobby...")
			client := s.netClient
			s.netClient = nil
			s.sceneChanger.ChangeScene(NewNetLobbyScene(s.sceneChanger, client))
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

	// Discover local level names for the level selector
	levelNames := discoverLevelNames()

	s.browserUI = ui.NewServerBrowserUI(
		func(address, level string) { s.onConnect(address, level) },
		func() { s.shouldGoBack = true },
		func() { s.fetchServers() },
		levelNames,
	)

	systems.PlayMusic(s.ecsWorld, cfg.Sound.MenuMusic)

	// Auto-fetch server list on scene entry
	s.fetchServers()
}

func (s *ServerBrowserScene) onConnect(address, level string) {
	if s.netClient != nil {
		s.netClient.Disconnect()
	}

	s.browserUI.SetStatus("Connecting...")
	s.browserUI.SetConnecting(true)

	s.netClient = network.NewClient()
	s.netClient.Connect(address, cfg.Network.GameVersion, "Player", level)
}

func (s *ServerBrowserScene) fetchServers() {
	s.browserUI.SetBrowseStatus("Fetching servers...")
	s.browserUI.SetRefreshing(true)

	go s.queryGgscaleFleet()
}

func (s *ServerBrowserScene) queryGgscaleFleet() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	address, err := network.FindMatch(ctx, "docker-default", "deathmatch")
	if err != nil {
		log.Printf("[browser] matchmaking failed: %v", err)
		s.recordFetchResult(nil, err)
		return
	}

	out := []ui.ServerEntry{{
		Name:       "Matchmade Server",
		Address:    address,
		Version:    cfg.Network.GameVersion,
		MaxPlayers: 4,
	}}
	s.recordFetchResult(out, nil)
}
func (s *ServerBrowserScene) recordFetchResult(servers []ui.ServerEntry, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetchedServers = servers
	s.fetchErr = err
	s.fetchDone = true
}

// discoverLevelNames returns sorted stem names of all .tmx levels in embedded assets.
func discoverLevelNames() []string {
	return assets.NewLevelLoader().ListLevelNames()
}
