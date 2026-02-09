package systems

import (
	"image/color"

	"github.com/automoto/doomerang-mp/assets"
	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

const (
	hudBarWidth  = 100
	hudBarHeight = 10
	hudMargin    = 10
	livesMargin  = 3
)


var heartIcon *ebiten.Image
var hudDrawOp = &ebiten.DrawImageOptions{}

// DrawHUD renders health bars and lives for all players.
// Players are positioned in corners based on their index:
// P1: top-left, P2: top-right, P3: bottom-left, P4: bottom-right
func DrawHUD(ecs *ecs.ECS, screen *ebiten.Image) {
	screenWidth := float32(cfg.C.Width)
	screenHeight := float32(cfg.C.Height)

	components.Player.Each(ecs.World, func(playerEntry *donburi.Entry) {
		playerData := components.Player.Get(playerEntry)
		hp := components.Health.Get(playerEntry)
		playerIndex := playerData.PlayerIndex

		// Calculate position based on player index
		var x, y float32
		switch playerIndex {
		case 0: // Top-left
			x, y = hudMargin, hudMargin
		case 1: // Top-right
			x, y = screenWidth-hudBarWidth-hudMargin, hudMargin
		case 2: // Bottom-left
			x, y = hudMargin, screenHeight-hudBarHeight-hudMargin-20
		case 3: // Bottom-right
			x, y = screenWidth-hudBarWidth-hudMargin, screenHeight-hudBarHeight-hudMargin-20
		default:
			return
		}

		// Get player color from config
		playerColor := cfg.PlayerColors.Colors[playerIndex%len(cfg.PlayerColors.Colors)].RGBA

		// Background (dark gray)
		vector.FillRect(screen, x, y,
			hudBarWidth, hudBarHeight,
			color.RGBA{40, 40, 40, 255}, false)

		// Current HP (player color)
		ratio := float32(hp.Current) / float32(hp.Max)
		vector.FillRect(screen, x, y,
			hudBarWidth*ratio, hudBarHeight,
			playerColor, false)

		// Draw lives counter
		drawPlayerLives(playerEntry, screen, x, y+hudBarHeight+livesMargin, playerIndex)
	})
}

func drawPlayerLives(playerEntry *donburi.Entry, screen *ebiten.Image, startX, startY float32, playerIndex int) {
	lives := components.Lives.Get(playerEntry)

	// Lazy load heart icon
	if heartIcon == nil {
		heartIcon = assets.GetIconImage("icon_heart.png")
	}

	heartWidth := heartIcon.Bounds().Dx()

	// For right-side players, draw hearts from right to left
	rightSide := playerIndex == 1 || playerIndex == 3

	for i := 0; i < lives.Lives; i++ {
		hudDrawOp.GeoM.Reset()
		var heartX float64
		if rightSide {
			heartX = float64(startX) + float64(hudBarWidth) - float64((i+1)*(heartWidth+livesMargin))
		} else {
			heartX = float64(startX) + float64(i*(heartWidth+livesMargin))
		}
		hudDrawOp.GeoM.Translate(heartX, float64(startY))
		screen.DrawImage(heartIcon, hudDrawOp)
	}
}
