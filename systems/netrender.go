package systems

import (
	"fmt"
	"image/color"
	"strconv"

	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/fonts"
	"github.com/automoto/doomerang-mp/shared/netcomponents"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/leap-fish/necs/esync"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

func DrawNetworkedPlayers(e *ecs.ECS, screen *ebiten.Image) {
	playerWidth := float32(cfg.Player.CollisionWidth)
	playerHeight := float32(cfg.Player.CollisionHeight)
	smallFont := fonts.ExcelSmall.Get()
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

		var rectColor color.RGBA
		if state != nil && state.IsLocal {
			rectColor = cfg.BrightGreen
		} else {
			idx := colorIndex % len(cfg.PlayerColors.Colors)
			rectColor = cfg.PlayerColors.Colors[idx].RGBA
			colorIndex++
		}

		x := float32(pos.X) - playerWidth/2
		y := float32(pos.Y) - playerHeight
		vector.DrawFilledRect(screen, x, y, playerWidth, playerHeight, rectColor, false)

		if state != nil {
			cx := float32(pos.X)
			cy := float32(pos.Y) - playerHeight/2
			dir := float32(state.Direction) * 6
			vector.DrawFilledRect(screen, cx+dir-2, cy-2, 4, 4, cfg.White, false)
		}

		if nid := esync.GetNetworkId(entry); nid != nil {
			label := "ID:" + strconv.Itoa(int(*nid))
			labelX := int(pos.X) - len(label)*3
			labelY := int(pos.Y) - int(playerHeight) - 6
			text.Draw(screen, label, smallFont, labelX, labelY, cfg.White)
		}
	})
}

func DrawNetworkHUD(e *ecs.ECS, screen *ebiten.Image) {
	entityCount := 0
	esync.NetworkEntityQuery.Each(e.World, func(_ *donburi.Entry) {
		entityCount++
	})

	info := fmt.Sprintf("Online - Entities: %d", entityCount)
	text.Draw(screen, info, fonts.ExcelSmall.Get(), 4, 12, cfg.LightGreen)
}
