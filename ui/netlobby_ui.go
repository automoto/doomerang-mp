package ui

import (
	"fmt"
	"image/color"

	"github.com/automoto/doomerang-mp/assets"
	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/golang/freetype/truetype"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font"
)

// NetLobbyUI holds the ebitenui interface for the network lobby
type NetLobbyUI struct {
	UI *ebitenui.UI

	// State
	Slots        [4]messages.LobbySlot
	GameMode     string
	MatchMinutes int
	LevelIndex   int
	HostID       uint32
	LocalNetID   uint32
	LevelNames   []string

	// Callbacks
	OnAction func(action messages.LobbyAction)
	OnGoBack func()

	// Widget references for updates
	slotButtons    [4]*widget.Button // Clicking our own slot cycles ready
	teamButtons    [4]*widget.Button // Team selection buttons (host only)
	gameModeLabel  *widget.Label
	levelLabel     *widget.Label
	gameModeButton *widget.Button
	levelButton    *widget.Button
	addBotButton   *widget.Button
	startButton    *widget.Button
	statusLabel    *widget.Label

	// Fonts
	titleFace  text.Face
	normalFace text.Face
	smallFace  text.Face

	initialized bool
}

func NewNetLobbyUI(localNetID uint32, levelNames []string, onAction func(messages.LobbyAction), onGoBack func()) *NetLobbyUI {
	lui := &NetLobbyUI{
		LocalNetID:   localNetID,
		LevelNames:   levelNames,
		OnAction:     onAction,
		OnGoBack:     onGoBack,
		GameMode:     "ffa",
		MatchMinutes: 2,
	}

	lui.loadFonts()
	lui.buildUI()

	return lui
}

func (lui *NetLobbyUI) loadFonts() {
	fontData, err := truetype.Parse(assets.ExcelFontTTF)
	if err != nil {
		panic(err)
	}

	opts := func(size float64) *truetype.Options {
		return &truetype.Options{Size: size, Hinting: font.HintingFull}
	}
	lui.titleFace = text.NewGoXFace(truetype.NewFace(fontData, opts(20)))
	lui.normalFace = text.NewGoXFace(truetype.NewFace(fontData, opts(12)))
	lui.smallFace = text.NewGoXFace(truetype.NewFace(fontData, opts(10)))
}

func (lui *NetLobbyUI) buildUI() {
	rootContainer := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.RGBA{20, 20, 30, 255})),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)

	contentContainer := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(8)),
			widget.RowLayoutOpts.Spacing(4),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
			}),
		),
	)

	titleLabel := widget.NewLabel(
		widget.LabelOpts.Text("MULTIPLAYER LOBBY", &lui.titleFace, &widget.LabelColor{
			Idle: color.RGBA{255, 255, 255, 255},
		}),
	)
	contentContainer.AddChild(titleLabel)

	// Slots
	slotsContainer := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(2),
		)),
	)
	for i := 0; i < 4; i++ {
		slotsContainer.AddChild(lui.buildSlotRow(i))
	}
	contentContainer.AddChild(slotsContainer)

	// Settings
	contentContainer.AddChild(lui.buildSettingsContainer())

	// Buttons
	buttonsContainer := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(10),
		)),
	)

	backButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(80, 28)),
		widget.ButtonOpts.Image(lui.buttonImage(color.RGBA{60, 60, 80, 255})),
		widget.ButtonOpts.Text("Leave", &lui.normalFace, &widget.ButtonTextColor{
			Idle: color.RGBA{255, 255, 255, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			if lui.OnGoBack != nil {
				lui.OnGoBack()
			}
		}),
	)
	buttonsContainer.AddChild(backButton)

	lui.startButton = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(100, 28)),
		widget.ButtonOpts.Image(lui.buttonImage(color.RGBA{40, 100, 40, 255})),
		widget.ButtonOpts.Text("START", &lui.normalFace, &widget.ButtonTextColor{
			Idle:     color.RGBA{255, 255, 255, 255},
			Disabled: color.RGBA{100, 100, 100, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			if lui.LocalNetID == lui.HostID {
				lui.OnAction(messages.LobbyAction{Action: "start_match"})
			}
		}),
	)
	buttonsContainer.AddChild(lui.startButton)
	contentContainer.AddChild(buttonsContainer)

	lui.statusLabel = widget.NewLabel(
		widget.LabelOpts.Text("", &lui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{255, 100, 100, 255},
		}),
	)
	contentContainer.AddChild(lui.statusLabel)

	rootContainer.AddChild(contentContainer)
	lui.UI = &ebitenui.UI{Container: rootContainer}
}

func (lui *NetLobbyUI) buildSlotRow(slotIndex int) *widget.Container {
	padding := widget.Insets{Top: 2, Bottom: 2, Left: 4, Right: 4}
	row := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.RGBA{40, 40, 50, 255})),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Padding(&padding),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	idx := slotIndex
	lui.slotButtons[slotIndex] = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(160, 20)),
		widget.ButtonOpts.Image(lui.buttonImage(color.RGBA{60, 60, 70, 255})),
		widget.ButtonOpts.Text(fmt.Sprintf("P%d: Empty", slotIndex+1), &lui.smallFace, &widget.ButtonTextColor{
			Idle: color.RGBA{255, 255, 255, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			slot := lui.Slots[idx]
			if slot.Type == 0 { // Empty
				lui.OnAction(messages.LobbyAction{Action: "pick_slot", Value: idx})
			} else if slot.PlayerID == lui.LocalNetID {
				if slot.Ready {
					lui.OnAction(messages.LobbyAction{Action: "unready"})
				} else {
					lui.OnAction(messages.LobbyAction{Action: "ready"})
				}
			} else if lui.LocalNetID == lui.HostID && slot.Type == 2 { // Bot
				lui.OnAction(messages.LobbyAction{Action: "remove_bot", Value: idx})
			}
		}),
	)
	row.AddChild(lui.slotButtons[slotIndex])

	lui.teamButtons[slotIndex] = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(50, 20)),
		widget.ButtonOpts.Image(lui.buttonImage(color.RGBA{60, 60, 70, 255})),
		widget.ButtonOpts.Text("Team", &lui.smallFace, &widget.ButtonTextColor{
			Idle: color.RGBA{255, 255, 255, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			// Team-cycling click handler is a placeholder; LobbyAction
			// for team change ships when host UI lands.
			_ = lui
		}),
	)
	row.AddChild(lui.teamButtons[slotIndex])

	return row
}

func (lui *NetLobbyUI) buildSettingsContainer() *widget.Container {
	container := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.RGBA{30, 30, 40, 255})),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(4)),
			widget.RowLayoutOpts.Spacing(3),
		)),
	)

	// Mode
	modeRow := widget.NewContainer(widget.ContainerOpts.Layout(widget.NewRowLayout(widget.RowLayoutOpts.Spacing(6))))
	modeRow.AddChild(widget.NewLabel(widget.LabelOpts.Text("Mode:", &lui.smallFace, &widget.LabelColor{Idle: color.White})))
	lui.gameModeLabel = widget.NewLabel(widget.LabelOpts.Text("FFA", &lui.smallFace, &widget.LabelColor{Idle: color.RGBA{255, 255, 100, 255}}))
	modeRow.AddChild(lui.gameModeLabel)
	lui.gameModeButton = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(50, 18)),
		widget.ButtonOpts.Image(lui.buttonImage(color.RGBA{60, 60, 80, 255})),
		widget.ButtonOpts.Text("Change", &lui.smallFace, &widget.ButtonTextColor{Idle: color.White}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			if lui.LocalNetID == lui.HostID {
				newMode := "ffa"
				switch lui.GameMode {
				case "ffa":
					newMode = "1v1"
				case "1v1":
					newMode = "2v2"
				case "2v2":
					newMode = "coop"
				}
				lui.OnAction(messages.LobbyAction{Action: "change_mode", String: newMode})
			}
		}),
	)
	modeRow.AddChild(lui.gameModeButton)
	container.AddChild(modeRow)

	// Add Bot
	lui.addBotButton = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(100, 20)),
		widget.ButtonOpts.Image(lui.buttonImage(color.RGBA{60, 60, 80, 255})),
		widget.ButtonOpts.Text("Add Bot", &lui.smallFace, &widget.ButtonTextColor{Idle: color.White}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			if lui.LocalNetID == lui.HostID {
				lui.OnAction(messages.LobbyAction{Action: "add_bot", Value: 1}) // Normal diff
			}
		}),
	)
	container.AddChild(lui.addBotButton)

	// Level
	levelRow := widget.NewContainer(widget.ContainerOpts.Layout(widget.NewRowLayout(widget.RowLayoutOpts.Spacing(6))))
	levelRow.AddChild(widget.NewLabel(widget.LabelOpts.Text("Level:", &lui.smallFace, &widget.LabelColor{Idle: color.White})))
	lui.levelLabel = widget.NewLabel(widget.LabelOpts.Text("Arena", &lui.smallFace, &widget.LabelColor{Idle: color.RGBA{255, 255, 100, 255}}))
	levelRow.AddChild(lui.levelLabel)
	lui.levelButton = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(50, 18)),
		widget.ButtonOpts.Image(lui.buttonImage(color.RGBA{60, 60, 80, 255})),
		widget.ButtonOpts.Text("Change", &lui.smallFace, &widget.ButtonTextColor{Idle: color.White}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			if lui.LocalNetID == lui.HostID && len(lui.LevelNames) > 0 {
				nextIdx := (lui.LevelIndex + 1) % len(lui.LevelNames)
				lui.OnAction(messages.LobbyAction{Action: "change_level", Value: nextIdx})
			}
		}),
	)
	levelRow.AddChild(lui.levelButton)
	container.AddChild(levelRow)

	return container
}

func (lui *NetLobbyUI) UpdateState(update messages.LobbyUpdate) {
	lui.Slots = update.Slots
	lui.GameMode = update.GameMode
	lui.MatchMinutes = update.MatchMinutes
	lui.LevelIndex = update.LevelIndex
	lui.HostID = update.HostID
	lui.UpdateUI()
}

func (lui *NetLobbyUI) UpdateUI() {
	if lui.startButton == nil {
		return // UI not fully built yet
	}

	isHost := lui.LocalNetID == lui.HostID

	for i := 0; i < 4; i++ {
		slot := lui.Slots[i]
		btn := lui.slotButtons[i]
		if btn == nil {
			continue
		}

		text := fmt.Sprintf("P%d: ", i+1)
		switch slot.Type {
		case 0:
			text += "Empty (Click to Join)"
		case 1:
			text += slot.Name
			if slot.Ready {
				text += " [READY]"
			} else {
				text += " [NOT READY]"
			}
			if slot.PlayerID == lui.LocalNetID {
				text += " (YOU)"
			}
		default:
			text += "BOT"
		}

		if txt := btn.Text(); txt != nil {
			txt.Label = text
		}

		if lui.teamButtons[i] != nil {
			lui.teamButtons[i].GetWidget().Disabled = !isHost || slot.Type == 0
		}
	}

	if lui.gameModeLabel != nil {
		lui.gameModeLabel.Label = lui.GameMode
	}
	if lui.gameModeButton != nil {
		lui.gameModeButton.GetWidget().Disabled = !isHost
	}
	if lui.addBotButton != nil {
		lui.addBotButton.GetWidget().Disabled = !isHost
	}

	if lui.levelLabel != nil && len(lui.LevelNames) > 0 {
		lui.levelLabel.Label = lui.LevelNames[lui.LevelIndex]
	}
	if lui.levelButton != nil {
		lui.levelButton.GetWidget().Disabled = !isHost
	}

	// Start button enabled only for host when all ready
	allReady := true
	humanCount := 0
	for _, s := range lui.Slots {
		if s.Type == 1 {
			humanCount++
			if !s.Ready {
				allReady = false
			}
		}
	}
	if lui.startButton != nil {
		lui.startButton.GetWidget().Disabled = !isHost || !allReady || humanCount == 0
	}

	if lui.statusLabel != nil {
		if !allReady {
			lui.statusLabel.Label = "Waiting for all players to be READY"
		} else if humanCount == 0 {
			lui.statusLabel.Label = "Need at least one human player"
		} else {
			lui.statusLabel.Label = ""
		}
	}
}

func (lui *NetLobbyUI) buttonImage(c color.RGBA) *widget.ButtonImage {
	idle := image.NewNineSliceColor(c)
	hover := image.NewNineSliceColor(color.RGBA{c.R + 20, c.G + 20, c.B + 20, 255})
	pressed := image.NewNineSliceColor(color.RGBA{c.R - 20, c.G - 20, c.B - 20, 255})
	return &widget.ButtonImage{Idle: idle, Hover: hover, Pressed: pressed}
}

func (lui *NetLobbyUI) Update() {
	lui.UI.Update()
	if !lui.initialized {
		lui.initialized = true
		lui.UpdateUI()
	}
}
