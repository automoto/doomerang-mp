package systems

import (
	"fmt"
	"image"
	"image/color"
	"strconv"

	"github.com/automoto/doomerang-mp/assets"
	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/fonts"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text" //nolint:staticcheck // TODO: migrate to text/v2
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/leap-fish/necs/esync"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

const (
	netHudBarWidth  = 100
	netHudBarHeight = 10
	netHudMargin    = 10
	netLivesMargin  = 3
)

var netHeartIcon *ebiten.Image
var netHudDrawOp = &ebiten.DrawImageOptions{}

// NewNetInterpSystem returns an interpolation system that uses the server's tick
// rate to compute the interpolation step dynamically instead of hardcoding 1/3.
func NewNetInterpSystem(tickRateFn func() int) func(*ecs.ECS) {
	cachedRate := 0
	cachedStep := 0.5 // default for 30 Hz

	return func(e *ecs.ECS) {
		if rate := tickRateFn(); rate != cachedRate && rate > 0 {
			cachedRate = rate
			cachedStep = float64(rate) / 60.0
		}
		interpStep := cachedStep

		esync.NetworkEntityQuery.Each(e.World, func(entry *donburi.Entry) {
			if !entry.HasComponent(components.NetInterp) {
				return
			}

			interp := components.NetInterp.Get(entry)

			// Boomerang interpolation
			if entry.HasComponent(netcomponents.NetBoomerang) {
				nb := netcomponents.NetBoomerang.Get(entry)
				if interp.T < 1.0 {
					interp.T += interpStep
					if interp.T > 1.0 {
						interp.T = 1.0
					}
					nb.X = interp.PrevX + (interp.TargetX-interp.PrevX)*interp.T
					nb.Y = interp.PrevY + (interp.TargetY-interp.PrevY)*interp.T
				} else if nb.VelX != 0 || nb.VelY != 0 {
					overshoot := (interp.T - 1.0) / interpStep
					if overshoot < cfg.Netcode.MaxExtrapFrames {
						interp.T += interpStep
						nb.X = interp.TargetX + nb.VelX*overshoot
						nb.Y = interp.TargetY + nb.VelY*overshoot
					}
				}
				return
			}

			// Player interpolation
			if !entry.HasComponent(netcomponents.NetPosition) {
				return
			}

			// Skip local player — prediction handles its position
			if entry.HasComponent(netcomponents.NetPlayerState) {
				if netcomponents.NetPlayerState.Get(entry).IsLocal {
					return
				}
			}

			pos := netcomponents.NetPosition.Get(entry)

			if interp.T < 1.0 {
				interp.T += interpStep
				if interp.T > 1.0 {
					interp.T = 1.0
				}
				pos.X = interp.PrevX + (interp.TargetX-interp.PrevX)*interp.T
				pos.Y = interp.PrevY + (interp.TargetY-interp.PrevY)*interp.T
			} else if interp.VelX != 0 || interp.VelY != 0 {
				// Extrapolate when past T=1.0 using velocity
				overshoot := (interp.T - 1.0) / interpStep
				if overshoot < cfg.Netcode.MaxExtrapFrames {
					interp.T += interpStep
					pos.X = interp.TargetX + interp.VelX*overshoot
					// Apply gravity to VelY during extrapolation for natural arcs
					extrapVelY := interp.VelY + cfg.Physics.Gravity*overshoot
					pos.Y = interp.TargetY + extrapVelY*overshoot
				}
			}
		})
	}
}

// UpdateNetAnimations advances animation frames for networked player entities
// and switches animation state based on NetPlayerState.StateID.
func UpdateNetAnimations(e *ecs.ECS) {
	esync.NetworkEntityQuery.Each(e.World, func(entry *donburi.Entry) {
		if !entry.HasComponent(netcomponents.NetPlayerState) || !entry.HasComponent(components.Animation) {
			return
		}

		state := netcomponents.NetPlayerState.Get(entry)
		animData := components.Animation.Get(entry)

		animData.SetAnimation(cfg.StateID(state.StateID))

		if animData.CurrentAnimation != nil {
			animData.CurrentAnimation.Update()
		}
	})
}

func DrawNetworkedPlayers(e *ecs.ECS, screen *ebiten.Image) {
	cameraEntry, ok := components.Camera.First(e.World)
	if !ok {
		return
	}
	camera := components.Camera.Get(cameraEntry)
	screenW := float64(screen.Bounds().Dx())
	screenH := float64(screen.Bounds().Dy())

	zoom := camera.Zoom
	if zoom == 0 {
		zoom = 1.0
	}

	collisionW := float64(cfg.Player.CollisionWidth)
	collisionH := float64(cfg.Player.CollisionHeight)

	esync.NetworkEntityQuery.Each(e.World, func(entry *donburi.Entry) {
		if !entry.HasComponent(netcomponents.NetPosition) {
			return
		}

		pos := netcomponents.NetPosition.Get(entry)
		hasState := entry.HasComponent(netcomponents.NetPlayerState)

		var state *netcomponents.NetPlayerStateData
		if hasState {
			state = netcomponents.NetPlayerState.Get(entry)
		}

		// Determine player color index for tinting
		playerColorIdx := 0
		if state != nil {
			playerColorIdx = state.PlayerIndex
		}

		// Try to render animated sprite
		hasAnim := entry.HasComponent(components.Animation)
		drewSprite := false

		if hasAnim {
			animData := components.Animation.Get(entry)
			if animData.CurrentAnimation != nil {
				frame := animData.CurrentAnimation.Frame()

				var img *ebiten.Image
				if frames, ok := animData.CachedFrames[animData.CurrentSheet]; ok {
					img = frames[frame]
				}

				// Fallback to runtime slicing
				if img == nil && animData.SpriteSheets[animData.CurrentSheet] != nil {
					sx := frame * animData.FrameWidth
					srcRect := image.Rect(sx, 0, sx+animData.FrameWidth, animData.FrameHeight)
					img = animData.SpriteSheets[animData.CurrentSheet].SubImage(srcRect).(*ebiten.Image)

					if animData.CachedFrames[animData.CurrentSheet] == nil {
						animData.CachedFrames[animData.CurrentSheet] = make(map[int]*ebiten.Image)
					}
					animData.CachedFrames[animData.CurrentSheet][frame] = img
				}

				if img != nil {
					drawOp.GeoM.Reset()
					drawOp.ColorScale.Reset()

					// Damage flash tinting
					if entry.HasComponent(components.Flash) {
						flash := components.Flash.Get(entry)
						if flash.Duration > 0 {
							drawOp.ColorScale.Scale(flash.R, flash.G, flash.B, 1)
							flash.Duration--
						}
					}

					// Bottom-center anchor (feet at collision box bottom-center)
					drawOp.GeoM.Translate(-float64(animData.FrameWidth)/2, -float64(animData.FrameHeight))

					if entry.HasComponent(components.SquashStretch) {
						ss := components.SquashStretch.Get(entry)
						drawOp.GeoM.Scale(ss.ScaleX, ss.ScaleY)
					}

					// Direction flip
					if state != nil && state.Direction < 0 {
						drawOp.GeoM.Scale(-1, 1)
					}

					// Position: NetPosition is top-left of collision box,
					// sprite anchors at bottom-center of collision box
					drawOp.GeoM.Translate(pos.X+collisionW/2, pos.Y+collisionH)

					// Camera transform
					drawOp.GeoM.Translate(-camera.Position.X, -camera.Position.Y)
					drawOp.GeoM.Scale(zoom, zoom)
					drawOp.GeoM.Translate(screenW/2, screenH/2)

					// Draw with shader for player color tinting
					if assets.TintShader != nil {
						drawPlayerWithShader(screen, img, drawOp, playerColorIdx)
					} else {
						screen.DrawImage(img, drawOp)
					}

					drewSprite = true
				}
			}
		}

		// Fallback to colored rectangle if no animation available
		if !drewSprite {
			var rectColor = cfg.PlayerColors.Colors[playerColorIdx%len(cfg.PlayerColors.Colors)].RGBA
			if state != nil && state.IsLocal {
				rectColor = cfg.BrightGreen
			}

			sx := (pos.X-camera.Position.X)*zoom + screenW/2
			sy := (pos.Y-camera.Position.Y)*zoom + screenH/2
			pw := float32(collisionW * zoom)
			ph := float32(collisionH * zoom)
			x := float32(sx) - pw/2
			y := float32(sy) - ph
			vector.FillRect(screen, x, y, pw, ph, rectColor, false)
		}

		if cfg.Debug.ShowNetworkDebug {
			// Direction indicator dot
			if state != nil {
				sx := (pos.X+collisionW/2-camera.Position.X)*zoom + screenW/2
				sy := (pos.Y+collisionH-camera.Position.Y)*zoom + screenH/2
				cy := float32(sy) - float32(collisionH*zoom)/2
				dir := float32(float64(state.Direction) * 6 * zoom)
				dotSize := float32(4 * zoom)
				vector.FillRect(screen, float32(sx)+dir-dotSize/2, cy-dotSize/2, dotSize, dotSize, cfg.White, false)
			}

			// Network ID label
			if nid := esync.GetNetworkId(entry); nid != nil {
				label := "ID:" + strconv.Itoa(int(*nid)) //nolint:gosec // NetworkId fits in int for the foreseeable player counts
				sx := (pos.X+collisionW/2-camera.Position.X)*zoom + screenW/2
				sy := (pos.Y-camera.Position.Y)*zoom + screenH/2
				labelX := int(sx) - len(label)*3
				labelY := int(sy) - 6
				text.Draw(screen, label, fonts.ExcelSmall.Get(), labelX, labelY, cfg.White)
			}
		}
	})
}

// boomerangRotation is a visual rotation counter for spinning boomerangs.
var boomerangRotation float64

// DrawNetworkedBoomerangs renders networked boomerang entities with a spinning sprite.
func DrawNetworkedBoomerangs(e *ecs.ECS, screen *ebiten.Image) {
	cameraEntry, ok := components.Camera.First(e.World)
	if !ok {
		return
	}
	camera := components.Camera.Get(cameraEntry)
	screenW := float64(screen.Bounds().Dx())
	screenH := float64(screen.Bounds().Dy())

	zoom := camera.Zoom
	if zoom == 0 {
		zoom = 1.0
	}

	boomerangRotation += 0.3

	esync.NetworkEntityQuery.Each(e.World, func(entry *donburi.Entry) {
		if !entry.HasComponent(netcomponents.NetBoomerang) || !entry.HasComponent(components.Sprite) {
			return
		}

		nb := netcomponents.NetBoomerang.Get(entry)
		sprite := components.Sprite.Get(entry)
		img := sprite.Image
		if img == nil {
			return
		}

		drawOp.GeoM.Reset()
		drawOp.ColorScale.Reset()

		// Center pivot
		w := float64(img.Bounds().Dx())
		h := float64(img.Bounds().Dy())
		drawOp.GeoM.Translate(-w/2, -h/2)

		// Spin
		drawOp.GeoM.Rotate(boomerangRotation)

		// Position at boomerang center
		drawOp.GeoM.Translate(nb.X+6, nb.Y+6)

		// Camera transform
		drawOp.GeoM.Translate(-camera.Position.X, -camera.Position.Y)
		drawOp.GeoM.Scale(zoom, zoom)
		drawOp.GeoM.Translate(screenW/2, screenH/2)

		screen.DrawImage(img, drawOp)
	})
}

func DrawNetworkHUD(e *ecs.ECS, screen *ebiten.Image) {
	// 1. Get Game State
	gameEntry, ok := netcomponents.NetGameState.First(e.World)
	if !ok {
		return
	}
	gs := netcomponents.NetGameState.Get(gameEntry)

	width := float64(screen.Bounds().Dx())
	height := float64(screen.Bounds().Dy())

	// 2. Draw HUD by Match State
	switch gs.MatchState {
	case netcomponents.MatchStateWaiting:
		drawWaitingMessage(screen, "WAITING FOR PLAYERS...", width, height)
	case netcomponents.MatchStateCountdown:
		drawNetworkCountdown(screen, gs.TimeRemaining, width, height)
	case netcomponents.MatchStatePlaying:
		drawNetworkTimer(screen, gs.TimeRemaining, width)
	case netcomponents.MatchStateRoundEnd:
		drawRoundEndOverlay(screen, e, gs, width, height)
	case netcomponents.MatchStateFinished:
		drawNetworkResults(screen, gs, width, height)
	}

	// 3. Draw Player Corner HUD (Health + Lives) for all active players
	if gs.MatchState == netcomponents.MatchStatePlaying || gs.MatchState == netcomponents.MatchStateCountdown || gs.MatchState == netcomponents.MatchStateRoundEnd {
		drawAllPlayersCornerHUD(e, screen, gs, width, height)
	}

	// 4. Debug Info
	if cfg.Debug.ShowNetworkDebug {
		entityCount := 0
		var ids []int
		esync.NetworkEntityQuery.Each(e.World, func(entry *donburi.Entry) {
			entityCount++
			if nid := esync.GetNetworkId(entry); nid != nil {
				ids = append(ids, int(*nid)) //nolint:gosec // NetworkId fits in int for the foreseeable player counts
			}
		})
		info := fmt.Sprintf("Online - Entities: %d  IDs: %v", entityCount, ids)
		text.Draw(screen, info, fonts.ExcelSmall.Get(), 4, 12, cfg.LightGreen)
	}
}

func drawWaitingMessage(screen *ebiten.Image, msg string, width, height float64) {
	fontFace := fonts.ExcelTitle.Get()
	textWidth := len(msg) * 24
	x := int(width/2) - textWidth/2
	y := int(height / 2)
	text.Draw(screen, msg, fontFace, x, y, cfg.BrightOrange)
}

func drawNetworkCountdown(screen *ebiten.Image, timeRemaining float64, width, height float64) {
	seconds := int(timeRemaining) + 1
	var countStr string
	if seconds > 0 {
		countStr = fmt.Sprintf("%d", seconds)
	} else {
		countStr = "GO!"
	}

	fontFace := fonts.ExcelTitle.Get()
	textWidth := len(countStr) * 24
	x := int(width/2) - textWidth/2
	y := int(height / 2)

	textColor := cfg.BrightOrange
	if seconds <= 0 {
		textColor = cfg.BrightGreen
	}

	// Overlay
	vector.FillRect(screen, 0, 0, float32(width), float32(height), color.RGBA{0, 0, 0, 100}, false)
	text.Draw(screen, countStr, fontFace, x, y, textColor)
}

func drawNetworkTimer(screen *ebiten.Image, timeRemaining float64, width float64) {
	seconds := int(timeRemaining)
	minutes := seconds / 60
	secs := seconds % 60
	timeStr := fmt.Sprintf("%d:%02d", minutes, secs)

	fontFace := fonts.ExcelBold.Get()
	timerWidth := float32(60)
	timerX := float32(width/2) - timerWidth/2
	vector.FillRect(screen, timerX, 5, timerWidth, 20, color.RGBA{0, 0, 0, 180}, false)

	textWidth := len(timeStr) * 8
	textX := int(width/2) - textWidth/2
	text.Draw(screen, timeStr, fontFace, textX, 20, cfg.White)
}

func drawAllPlayersCornerHUD(e *ecs.ECS, screen *ebiten.Image, gs *netcomponents.NetGameStateData, screenWidth, screenHeight float64) {
	netcomponents.NetPlayerState.Each(e.World, func(entry *donburi.Entry) {
		state := netcomponents.NetPlayerState.Get(entry)
		playerIndex := state.PlayerIndex

		// Calculate position based on player index (4 corners)
		var x, y float32
		switch playerIndex {
		case 0: // Top-left
			x, y = netHudMargin, netHudMargin
		case 1: // Top-right
			x, y = float32(screenWidth)-netHudBarWidth-netHudMargin, netHudMargin
		case 2: // Bottom-left
			x, y = netHudMargin, float32(screenHeight)-netHudBarHeight-netHudMargin-20
		case 3: // Bottom-right
			x, y = float32(screenWidth)-netHudBarWidth-netHudMargin, float32(screenHeight)-netHudBarHeight-netHudMargin-20
		default:
			return
		}

		// Get player color from config
		playerColor := cfg.PlayerColors.Colors[playerIndex%len(cfg.PlayerColors.Colors)].RGBA

		// Background (dark gray)
		vector.FillRect(screen, x, y, netHudBarWidth, netHudBarHeight, color.RGBA{40, 40, 40, 255}, false)

		// Current HP (player color)
		hpRatio := float32(state.Health) / float32(cfg.Player.Health)
		if hpRatio < 0 {
			hpRatio = 0
		}
		vector.FillRect(screen, x, y, netHudBarWidth*hpRatio, netHudBarHeight, playerColor, false)

		// Draw lives counter
		drawNetworkPlayerLives(state.Lives, screen, x, y+netHudBarHeight+netLivesMargin, playerIndex)

		// Draw Round Win pips
		drawRoundPips(screen, x, y+netHudBarHeight+netLivesMargin+15, gs.RoundWins[gs.SlotTeams[playerIndex]], gs.RoundsToWin, playerColor, playerIndex >= 2)
	})
}

func drawNetworkPlayerLives(lives int, screen *ebiten.Image, startX, startY float32, playerIndex int) {
	if netHeartIcon == nil {
		netHeartIcon = assets.GetIconImage("icon_heart.png")
	}
	if netHeartIcon == nil {
		return
	}

	heartWidth := netHeartIcon.Bounds().Dx()
	rightSide := playerIndex == 1 || playerIndex == 3

	for i := 0; i < lives; i++ {
		netHudDrawOp.GeoM.Reset()
		var heartX float64
		if rightSide {
			heartX = float64(startX) + float64(netHudBarWidth) - float64((i+1)*(heartWidth+netLivesMargin))
		} else {
			heartX = float64(startX) + float64(i*(heartWidth+netLivesMargin))
		}
		netHudDrawOp.GeoM.Translate(heartX, float64(startY))
		screen.DrawImage(netHeartIcon, netHudDrawOp)
	}
}

func drawRoundPips(screen *ebiten.Image, x, y float32, wins, toWin int, teamColor color.RGBA, bottom bool) {
	const pipSize = 6
	const pipGap = 4

	cx := x
	if x > 320 { // Right side
		cx = x + netHudBarWidth - float32(toWin*(pipSize+pipGap))
	}

	for p := 0; p < toWin; p++ {
		if p < wins {
			vector.FillRect(screen, cx, y, pipSize, pipSize, teamColor, false)
		} else {
			vector.FillRect(screen, cx, y, pipSize, pipSize, color.RGBA{60, 60, 60, 255}, false)
		}
		cx += pipSize + pipGap
	}
}

// drawRoundEndOverlay shows a semi-transparent overlay with round winner info.
func drawRoundEndOverlay(screen *ebiten.Image, e *ecs.ECS, gs *netcomponents.NetGameStateData, width, height float64) {
	// Semi-transparent overlay
	vector.FillRect(screen, 0, 0, float32(width), float32(height), color.RGBA{0, 0, 0, 140}, false)

	titleFont := fonts.ExcelTitle.Get()
	roundStr := fmt.Sprintf("ROUND %d", gs.CurrentRound)
	text.Draw(screen, roundStr, titleFont, int(width/2)-len(roundStr)*12, int(height/2)-20, cfg.BrightOrange)

	// Winner name
	winnerName := ""
	if gs.WinnerID != 0 {
		winnerName = getNetworkPlayerName(e, gs.WinnerID)
	}

	if winnerName != "" {
		winStr := winnerName + " wins the round!"
		text.Draw(screen, winStr, fonts.ExcelBold.Get(), int(width/2)-len(winStr)*4, int(height/2)+15, cfg.Yellow)
	} else {
		text.Draw(screen, "Draw!", fonts.ExcelBold.Get(), int(width/2)-20, int(height/2)+15, cfg.White)
	}
}

func drawNetworkResults(screen *ebiten.Image, gs *netcomponents.NetGameStateData, width, height float64) {
	// Full dark overlay
	vector.FillRect(screen, 0, 0, float32(width), float32(height), color.RGBA{0, 0, 0, 200}, false)

	titleFont := fonts.ExcelTitle.Get()
	title := "MATCH OVER"
	text.Draw(screen, title, titleFont, int(width/2)-100, 60, cfg.BrightOrange)

	// Winner info
	winnerName := ""
	for i := 0; i < 4; i++ {
		if gs.SlotNetIDs[i] == gs.WinnerID && gs.SlotTypes[i] != 0 {
			winnerName = gs.SlotNames[i]
			break
		}
	}
	winnerStr := "TIE!"
	if winnerName != "" {
		winnerStr = winnerName + " WINS!"
	}
	text.Draw(screen, winnerStr, fonts.ExcelBold.Get(), int(width/2)-len(winnerStr)*4, 100, cfg.Yellow)

	// Show round wins per slot
	y := 130
	for slotIdx := 0; slotIdx < 4; slotIdx++ {
		if gs.SlotTypes[slotIdx] == 0 {
			continue
		}
		name := gs.SlotNames[slotIdx]
		if name == "" {
			name = "P" + strconv.Itoa(slotIdx+1)
		}
		team := gs.SlotTeams[slotIdx]
		roundWins := 0
		if gs.RoundWins != nil {
			roundWins = gs.RoundWins[team]
		}
		kos := 0
		if gs.Scores != nil {
			kos = gs.Scores[gs.SlotNetIDs[slotIdx]]
		}

		playerColor := cfg.PlayerColors.Colors[slotIdx%len(cfg.PlayerColors.Colors)].RGBA
		line := fmt.Sprintf("%s  Rounds: %d  KOs: %d", name, roundWins, kos)
		text.Draw(screen, line, fonts.ExcelBold.Get(), int(width/2)-len(line)*4, y, playerColor)
		y += 22
	}
}

func getNetworkPlayerName(e *ecs.ECS, netID uint32) string {
	if netID == 0 {
		return "None"
	}
	entity := esync.FindByNetworkId(e.World, esync.NetworkId(netID))
	if !e.World.Valid(entity) {
		return "P" + strconv.Itoa(int(netID))
	}

	entry := e.World.Entry(entity)
	if entry.HasComponent(netcomponents.NetPlayerState) {
		state := netcomponents.NetPlayerState.Get(entry)
		if state.IsBot {
			return "[BOT]"
		}
	}

	return "Player " + strconv.Itoa(int(netID))
}
