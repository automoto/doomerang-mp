package systems

import (
	"fmt"
	"image/color"

	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/fonts"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text" //nolint:staticcheck // TODO: migrate to text/v2
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/yohamta/donburi/ecs"
)


// DrawMatchHUD renders match-specific UI elements (timer, scores, countdown, results)
func DrawMatchHUD(ecs *ecs.ECS, screen *ebiten.Image) {
	matchEntry, ok := components.Match.First(ecs.World)
	if !ok {
		return
	}
	match := components.Match.Get(matchEntry)

	switch match.State {
	case cfg.MatchStateCountdown:
		drawCountdown(screen, match)
	case cfg.MatchStatePlaying:
		drawMatchTimer(screen, match)
		drawMatchScores(screen, match)
	case cfg.MatchStateFinished:
		drawMatchResults(screen, match)
	}
}

func drawMatchTimer(screen *ebiten.Image, match *components.MatchData) {
	width := float64(cfg.C.Width)
	fontFace := fonts.ExcelBold.Get()

	// Calculate remaining time
	seconds := match.Timer / 60
	minutes := seconds / 60
	secs := seconds % 60

	// Format time string
	timeStr := fmt.Sprintf("%d:%02d", minutes, secs)

	// Draw timer background
	timerWidth := float32(60)
	timerHeight := float32(20)
	timerX := float32(width/2) - timerWidth/2
	timerY := float32(5)

	vector.FillRect(screen, timerX, timerY, timerWidth, timerHeight,
		color.RGBA{0, 0, 0, 180}, false)

	// Draw timer text centered
	textWidth := len(timeStr) * 8
	textX := int(width/2) - textWidth/2
	text.Draw(screen, timeStr, fontFace, textX, 20, cfg.White)
}

func drawMatchScores(screen *ebiten.Image, match *components.MatchData) {
	width := float64(cfg.C.Width)
	fontFace := fonts.ExcelSmall.Get()

	// Draw scores below timer
	startY := 30
	spacing := 12

	for i, score := range match.Scores {
		playerColor := cfg.PlayerColors.Colors[i%len(cfg.PlayerColors.Colors)].RGBA
		scoreStr := fmt.Sprintf("P%d: %d", i+1, score.KOs)

		// Position scores in a row centered under timer
		numScores := len(match.Scores)
		totalWidth := numScores * 40
		startX := int(width/2) - totalWidth/2
		x := startX + i*40

		text.Draw(screen, scoreStr, fontFace, x, startY+spacing, playerColor)
	}
}

func drawCountdown(screen *ebiten.Image, match *components.MatchData) {
	width := float64(cfg.C.Width)
	height := float64(cfg.C.Height)
	fontFace := fonts.ExcelTitle.Get()

	// Semi-transparent overlay
	vector.FillRect(screen, 0, 0, float32(width), float32(height),
		color.RGBA{0, 0, 0, 120}, false)

	// Countdown text
	var countStr string
	if match.CountdownValue > 0 {
		countStr = fmt.Sprintf("%d", match.CountdownValue)
	} else {
		countStr = "GO!"
	}

	// Center the countdown text
	textWidth := len(countStr) * 24
	x := int(width/2) - textWidth/2
	y := int(height / 2)

	// Draw with a bright color
	textColor := cfg.BrightOrange
	if match.CountdownValue <= 0 {
		textColor = cfg.BrightGreen
	}

	text.Draw(screen, countStr, fontFace, x, y, textColor)
}

func drawMatchResults(screen *ebiten.Image, match *components.MatchData) {
	width := float64(cfg.C.Width)
	height := float64(cfg.C.Height)
	fontFace := fonts.ExcelBold.Get()
	titleFont := fonts.ExcelTitle.Get()

	// Full overlay
	vector.FillRect(screen, 0, 0, float32(width), float32(height),
		color.RGBA{0, 0, 0, 200}, false)

	// Title
	title := "MATCH OVER"
	titleWidth := len(title) * 20
	titleX := int(width/2) - titleWidth/2
	text.Draw(screen, title, titleFont, titleX, 60, cfg.BrightOrange)

	// Winner announcement
	var winnerStr string
	var winnerColor color.RGBA

	switch {
	case match.GameMode == cfg.GameMode2v2 && match.WinningTeam >= 0:
		winnerStr = fmt.Sprintf("Team %d Wins!", match.WinningTeam+1)
		winnerColor = cfg.BrightGreen
	case match.WinnerIndex >= 0:
		winnerStr = fmt.Sprintf("Player %d Wins!", match.WinnerIndex+1)
		winnerColor = cfg.PlayerColors.Colors[match.WinnerIndex%len(cfg.PlayerColors.Colors)].RGBA
	case match.WinnerIndex == -1:
		winnerStr = "It's a Tie!"
		winnerColor = cfg.Yellow
	default:
		winnerStr = "Match Complete"
		winnerColor = cfg.White
	}

	winnerWidth := len(winnerStr) * 12
	winnerX := int(width/2) - winnerWidth/2
	text.Draw(screen, winnerStr, fontFace, winnerX, 100, winnerColor)

	// Score table
	y := 140
	for i, score := range match.Scores {
		playerColor := cfg.PlayerColors.Colors[i%len(cfg.PlayerColors.Colors)].RGBA
		scoreStr := fmt.Sprintf("P%d: %d KOs / %d Deaths", i+1, score.KOs, score.Deaths)
		scoreWidth := len(scoreStr) * 8
		x := int(width/2) - scoreWidth/2
		text.Draw(screen, scoreStr, fontFace, x, y, playerColor)
		y += 25
	}

	// Return hint
	hint := "Returning to menu..."
	hintWidth := len(hint) * 7
	hintX := int(width/2) - hintWidth/2
	text.Draw(screen, hint, fontFace, hintX, int(height)-30, cfg.White)
}
