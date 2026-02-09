package main

import (
	_ "embed"
	"image"
	"log"

	"github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/fonts"
	"github.com/automoto/doomerang-mp/scenes"
	"github.com/automoto/doomerang-mp/shared/protocol"
	"github.com/automoto/doomerang-mp/systems"
	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed assets/fonts/excel.ttf
var excelFont []byte

type Scene interface {
	Update()
	Draw(screen *ebiten.Image)
}

type Game struct {
	bounds image.Rectangle
	scene  Scene
}

// ChangeScene switches to a new scene
func (g *Game) ChangeScene(scene interface{}) {
	g.scene = scene.(Scene)
}

func NewGame() *Game {
	fonts.LoadFont(fonts.Excel, excelFont)
	fonts.LoadFontWithSize(fonts.ExcelBold, excelFont, 20)
	fonts.LoadFontWithSize(fonts.ExcelTitle, excelFont, 32)
	fonts.LoadFontWithSize(fonts.ExcelSmall, excelFont, 12)

	g := &Game{
		bounds: image.Rectangle{},
	}

	if config.Debug.SkipMenu {
		g.scene = scenes.NewPlatformerScene(g)
	} else {
		g.scene = scenes.NewMenuScene(g)
	}

	return g
}

func (g *Game) Update() error {
	g.scene.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.scene.Draw(screen)
}

func (g *Game) Layout(width, height int) (int, int) {
	g.bounds = image.Rect(0, 0, config.C.Width, config.C.Height)
	return config.C.Width, config.C.Height
}

func main() {
	// Start pprof server for memory profiling
	// Usage: go tool pprof http://localhost:6060/debug/pprof/heap
	// go func() {
	// 	log.Println("pprof server running on http://localhost:6060/debug/pprof/")
	// 	if err := http.ListenAndServe("localhost:6060", nil); err != nil {
	// 		log.Printf("pprof server error: %v", err)
	// 	}
	// }()

	// Register network components for client-side deserialization
	if err := protocol.RegisterComponents(); err != nil {
		log.Fatalf("Failed to register network components: %v", err)
	}

	ebiten.SetWindowSize(config.C.Width, config.C.Height)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeOnlyFullscreenEnabled)

	// Initialize persistence and load saved settings
	if err := systems.InitPersistence(); err != nil {
		log.Printf("Warning: Could not initialize persistence: %v", err)
	}
	if saved, err := systems.LoadSettings(); err == nil && saved != nil {
		systems.ApplySavedSettingsGlobal(saved)
	}

	if err := ebiten.RunGame(NewGame()); err != nil {
		log.Fatal(err)
	}
}
