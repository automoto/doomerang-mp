package scenes

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"net/http"
	"sync"
	"time"

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
	httpClient     *http.Client
}

func NewServerBrowserScene(sc SceneChanger) *ServerBrowserScene {
	return &ServerBrowserScene{
		sceneChanger: sc,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
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
			s.browserUI.SetStatus("Joined! Loading game...")
			client := s.netClient
			s.netClient = nil
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
		func() { s.fetchServers() },
	)

	systems.PlayMusic(s.ecsWorld, cfg.Sound.MenuMusic)

	// Auto-fetch server list on scene entry
	s.fetchServers()
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

func (s *ServerBrowserScene) fetchServers() {
	s.browserUI.SetBrowseStatus("Fetching servers...")
	s.browserUI.SetRefreshing(true)

	go s.queryMasterServer()
}

func (s *ServerBrowserScene) queryMasterServer() {
	resp, err := s.httpClient.Get(cfg.Network.MasterServerURL + "/servers")
	if err != nil {
		log.Printf("[browser] master server query failed: %v", err)
		s.mu.Lock()
		s.fetchErr = err
		s.fetchDone = true
		s.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("master server returned status %d", resp.StatusCode)
		log.Printf("[browser] %v", err)
		s.mu.Lock()
		s.fetchErr = err
		s.fetchDone = true
		s.mu.Unlock()
		return
	}

	var servers []ui.ServerEntry
	if err := json.NewDecoder(resp.Body).Decode(&servers); err != nil {
		log.Printf("[browser] failed to decode server list: %v", err)
		s.mu.Lock()
		s.fetchErr = err
		s.fetchDone = true
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	s.fetchedServers = servers
	s.fetchDone = true
	s.mu.Unlock()
}
