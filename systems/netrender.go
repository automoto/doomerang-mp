package systems

import (
	"fmt"
	"image"
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
	colorIndex := 0

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
		isLocal := state != nil && state.IsLocal
		playerColorIdx := 0
		if isLocal {
			playerColorIdx = 0
		} else {
			playerColorIdx = colorIndex % len(cfg.PlayerColors.Colors)
			colorIndex++
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
			if isLocal {
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
				label := "ID:" + strconv.Itoa(int(*nid))
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
	entityCount := 0
	var ids []int
	esync.NetworkEntityQuery.Each(e.World, func(entry *donburi.Entry) {
		entityCount++
		if nid := esync.GetNetworkId(entry); nid != nil {
			ids = append(ids, int(*nid))
		}
	})

	info := fmt.Sprintf("Online - Entities: %d  IDs: %v", entityCount, ids)
	text.Draw(screen, info, fonts.ExcelSmall.Get(), 4, 12, cfg.LightGreen)
}
