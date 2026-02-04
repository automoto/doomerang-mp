package scenes

import (
	"image/color"
	"sync"

	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/systems"
	"github.com/automoto/doomerang-mp/ui"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

// LobbyScene displays the match setup lobby using ebitenui
type LobbyScene struct {
	ecs          *ecs.ECS
	sceneChanger SceneChanger
	lobbyUI      *ui.LobbyUI
	lobbyData    *components.LobbyData
	once         sync.Once
	shouldStart  bool
	shouldGoBack bool
}

// NewLobbyScene creates a new lobby scene
func NewLobbyScene(sc SceneChanger) *LobbyScene {
	return &LobbyScene{sceneChanger: sc}
}

func (ls *LobbyScene) Update() {
	ls.once.Do(ls.configure)

	// Update ECS for audio
	ls.ecs.Update()

	// Update ebitenui
	ls.lobbyUI.Update()

	// Handle scene transitions
	if ls.shouldStart {
		ls.startMatch()
		return
	}
	if ls.shouldGoBack {
		systems.FadeOutMusic(ls.ecs)
		ls.sceneChanger.ChangeScene(NewMenuScene(ls.sceneChanger))
		return
	}
}

func (ls *LobbyScene) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 20, 30, 255})

	if ls.ecs == nil {
		return
	}

	// Draw ebitenui
	ls.lobbyUI.UI.Draw(screen)
}

func (ls *LobbyScene) configure() {
	ls.ecs = ecs.NewECS(donburi.NewWorld())

	// Audio system
	ls.ecs.AddSystem(systems.UpdateAudio)

	// Initialize lobby data
	ls.lobbyData = &components.LobbyData{}
	systems.InitLobby(ls.lobbyData)

	// Create ebitenui lobby
	ls.lobbyUI = ui.NewLobbyUI(
		ls.lobbyData,
		func() { ls.shouldStart = true },
		func() { ls.shouldGoBack = true },
	)

	// Continue playing menu music
	systems.PlayMusic(ls.ecs, cfg.Sound.MenuMusic)
}

// startMatch creates the game scene with the lobby configuration
func (ls *LobbyScene) startMatch() {
	systems.FadeOutMusic(ls.ecs)

	// Create match config from lobby data
	matchConfig := &MatchConfig{
		Slots:        ls.lobbyData.Slots,
		GameMode:     ls.lobbyData.GameMode,
		MatchMinutes: ls.lobbyData.MatchMinutes,
	}

	ls.sceneChanger.ChangeScene(NewPlatformerSceneWithConfig(ls.sceneChanger, matchConfig))
}

// MatchConfig holds configuration passed from lobby to game
type MatchConfig struct {
	Slots        [4]components.PlayerSlot
	GameMode     cfg.GameModeID
	MatchMinutes int
}
