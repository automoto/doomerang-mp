package main

import (
	"context"
	_ "embed"
	"fmt"
	"image"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/fonts"
	"github.com/automoto/doomerang-mp/network"
	"github.com/automoto/doomerang-mp/scenes"
	"github.com/automoto/doomerang-mp/shared/protocol"
	"github.com/automoto/doomerang-mp/systems"
	ggscale "github.com/automoto/ggscale-go"
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

	// Optional ggscale integration. If GGSCALE_API_KEY is unset the
	// game runs as before. When set, log in once at startup and
	// register the resulting client; every subsequent network.NewClient
	// will pick it up via network.SharedGgscale.
	if err := initGgscale(); err != nil {
		log.Fatalf("ggscale init: %v", err)
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

// initGgscale reads GGSCALE_* env vars; on a non-empty
// GGSCALE_PUBLISHABLE_KEY it builds an SDK client, registers
// anonymously (resuming the persisted identity if one is on disk), and
// stashes the result for the network client to attach to JoinRequest.
//
// Required env vars when GGSCALE_PUBLISHABLE_KEY is set:
//   - GGSCALE_LEADERBOARD_ID: int64 ID of the leaderboard scores
//     should be posted to. (Create one with `INSERT INTO leaderboards
//     ...` until the dashboard grows a UI for it.)
//
// Optional:
//   - GGSCALE_BASE_URL: defaults to http://localhost:8080.
//
// Why publishable, not secret: this credential ships embedded in the
// game binary. Publishable keys can register an anonymous session and
// read leaderboards; the score-submission write is gated to secret-tier
// keys server-side and runs from the dedicated game server, which has
// its own GGSCALE_SECRET_KEY.
func initGgscale() error {
	apiKey := os.Getenv("GGSCALE_PUBLISHABLE_KEY")
	if apiKey == "" {
		return nil
	}

	baseURL := os.Getenv("GGSCALE_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	lbStr := os.Getenv("GGSCALE_LEADERBOARD_ID")
	if lbStr == "" {
		return fmt.Errorf("GGSCALE_LEADERBOARD_ID is required when GGSCALE_PUBLISHABLE_KEY is set")
	}
	lbID, err := strconv.ParseInt(lbStr, 10, 64)
	if err != nil {
		return fmt.Errorf("GGSCALE_LEADERBOARD_ID must be an integer: %w", err)
	}

	storePath := ggscale.DefaultSessionPath("doomerang-mp")
	transport := &ggscale.StdNetTransport{BaseURL: baseURL}
	auth := ggscale.NewAnonymousAuth(transport, apiKey, storePath)

	gg, err := ggscale.NewClient(ggscale.Options{
		Transport:       transport,
		APIKey:          apiKey,
		OnSessionUpdate: auth.SaveSession,
	})
	if err != nil {
		return fmt.Errorf("ggscale.NewClient: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := gg.Login(ctx, auth); err != nil {
		return fmt.Errorf("ggscale anonymous login: %w", err)
	}

	network.SetSharedGgscale(gg, lbID)
	log.Printf("[ggscale] authenticated anonymously as end_user_id=%d, leaderboard=%d, session=%s",
		gg.Session().EndUserID, lbID, storePath)
	return nil
}
