package ui

import (
	"bytes"
	"fmt"
	"image/color"

	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/systems"
	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

// LobbyUI holds the ebitenui interface for the lobby
type LobbyUI struct {
	UI    *ebitenui.UI
	Lobby *components.LobbyData

	// Callbacks
	OnStartMatch func()
	OnGoBack     func()

	// Widget references for updates
	slotLabels      [4]*widget.Label
	slotTypeButtons [4]*widget.Button
	schemeButtons   [4]*widget.Button // Control scheme buttons
	diffLabels      [4]*widget.Label
	teamLabels      [4]*widget.Label
	gameModeLabel   *widget.Label
	matchTimeLabel  *widget.Label
	startButton     *widget.Button
	statusLabel     *widget.Label

	// Fonts (stored as interface for ebitenui compatibility)
	titleFace  text.Face
	normalFace text.Face
	smallFace  text.Face
}

// Player colors for visual identification
var playerColors = []color.Color{
	color.RGBA{0, 200, 0, 255},   // P1: Green
	color.RGBA{0, 150, 255, 255}, // P2: Blue
	color.RGBA{255, 200, 0, 255}, // P3: Yellow
	color.RGBA{255, 100, 0, 255}, // P4: Orange
}

// NewLobbyUI creates a new lobby UI with ebitenui
func NewLobbyUI(lobby *components.LobbyData, onStartMatch, onGoBack func()) *LobbyUI {
	lui := &LobbyUI{
		Lobby:        lobby,
		OnStartMatch: onStartMatch,
		OnGoBack:     onGoBack,
	}

	lui.loadFonts()
	lui.buildUI()

	return lui
}

func (lui *LobbyUI) loadFonts() {
	// Load fonts using go fonts
	fontSource, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		panic(err)
	}

	// Store as text.Face interface for ebitenui compatibility
	// Smaller fonts to fit 640x360 screen
	lui.titleFace = &text.GoTextFace{
		Source: fontSource,
		Size:   18,
	}
	lui.normalFace = &text.GoTextFace{
		Source: fontSource,
		Size:   12,
	}
	lui.smallFace = &text.GoTextFace{
		Source: fontSource,
		Size:   10,
	}
}

func (lui *LobbyUI) buildUI() {
	// Root container with AnchorLayout to fill the screen
	rootContainer := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.RGBA{20, 20, 30, 255})),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)

	// Content container with vertical layout, centered - very compact for 640x360 screen
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

	// Title
	titleLabel := widget.NewLabel(
		widget.LabelOpts.Text("MATCH SETUP", &lui.titleFace, &widget.LabelColor{
			Idle: color.RGBA{255, 255, 255, 255},
		}),
	)
	contentContainer.AddChild(titleLabel)

	// Player slots container
	slotsContainer := lui.buildSlotsContainer()
	contentContainer.AddChild(slotsContainer)

	// Settings container
	settingsContainer := lui.buildSettingsContainer()
	contentContainer.AddChild(settingsContainer)

	// Bottom buttons
	buttonsContainer := lui.buildButtonsContainer()
	contentContainer.AddChild(buttonsContainer)

	// Status label
	lui.statusLabel = widget.NewLabel(
		widget.LabelOpts.Text("", &lui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{255, 100, 100, 255},
		}),
	)
	contentContainer.AddChild(lui.statusLabel)

	rootContainer.AddChild(contentContainer)

	// Create UI
	lui.UI = &ebitenui.UI{
		Container: rootContainer,
	}
	// Note: Don't call UpdateUI() here - widgets aren't validated yet
}

func (lui *LobbyUI) buildSlotsContainer() *widget.Container {
	container := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(2),
		)),
	)

	for i := 0; i < 4; i++ {
		slotRow := lui.buildSlotRow(i)
		container.AddChild(slotRow)
	}

	return container
}

func (lui *LobbyUI) buildSlotRow(slotIndex int) *widget.Container {
	slot := &lui.Lobby.Slots[slotIndex]

	padding := widget.Insets{Top: 2, Bottom: 2, Left: 4, Right: 4}
	row := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.RGBA{40, 40, 50, 255})),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Padding(&padding),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	// Player label (P1, P2, etc.)
	playerLabel := widget.NewLabel(
		widget.LabelOpts.Text(fmt.Sprintf("P%d:", slotIndex+1), &lui.normalFace, &widget.LabelColor{
			Idle: playerColors[slotIndex],
		}),
	)
	row.AddChild(playerLabel)

	// Slot type button (Empty/Human/Bot) - use initial value from lobby
	idx := slotIndex // Capture for closure
	initialTypeText := systems.GetSlotTypeName(slot.Type)
	typeButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(70, 20),
		),
		widget.ButtonOpts.Image(lui.buttonImage()),
		widget.ButtonOpts.Text(initialTypeText, &lui.smallFace, &widget.ButtonTextColor{
			Idle:    color.RGBA{255, 255, 255, 255},
			Hover:   color.RGBA{255, 255, 200, 255},
			Pressed: color.RGBA{200, 200, 200, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			systems.CycleSlotType(lui.Lobby, idx)
			lui.UpdateUI()
		}),
	)
	lui.slotTypeButtons[slotIndex] = typeButton
	row.AddChild(typeButton)

	// Control scheme button (for human keyboard players)
	initialSchemeText := systems.GetInputDeviceName(slot)
	schemeButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(90, 20),
		),
		widget.ButtonOpts.Image(lui.buttonImage()),
		widget.ButtonOpts.Text(initialSchemeText, &lui.smallFace, &widget.ButtonTextColor{
			Idle:     color.RGBA{180, 180, 180, 255},
			Hover:    color.RGBA{255, 255, 200, 255},
			Pressed:  color.RGBA{200, 200, 200, 255},
			Disabled: color.RGBA{100, 100, 100, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			systems.CycleControlScheme(lui.Lobby, idx)
			lui.UpdateUI()
		}),
	)
	lui.schemeButtons[slotIndex] = schemeButton
	row.AddChild(schemeButton)

	// Slot label (removed - now using scheme button)
	lui.slotLabels[slotIndex] = widget.NewLabel(
		widget.LabelOpts.Text("", &lui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{180, 180, 180, 255},
		}),
	)

	// Difficulty label (for bots)
	initialDiffLabel := ""
	if slot.Type == components.SlotBot {
		initialDiffLabel = systems.GetBotDifficultyName(slot.BotDifficulty)
	}
	lui.diffLabels[slotIndex] = widget.NewLabel(
		widget.LabelOpts.Text(initialDiffLabel, &lui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{180, 180, 180, 255},
		}),
	)
	row.AddChild(lui.diffLabels[slotIndex])

	// Team label
	lui.teamLabels[slotIndex] = widget.NewLabel(
		widget.LabelOpts.Text("", &lui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{180, 180, 180, 255},
		}),
	)
	row.AddChild(lui.teamLabels[slotIndex])

	return row
}

func (lui *LobbyUI) buildSettingsContainer() *widget.Container {
	padding := widget.Insets{Top: 4, Bottom: 4, Left: 6, Right: 6}
	container := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.RGBA{30, 30, 40, 255})),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(&padding),
			widget.RowLayoutOpts.Spacing(3),
		)),
	)

	// Settings title
	settingsTitle := widget.NewLabel(
		widget.LabelOpts.Text("SETTINGS", &lui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{200, 200, 255, 255},
		}),
	)
	container.AddChild(settingsTitle)

	// Game mode row
	modeRow := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	modeLabel := widget.NewLabel(
		widget.LabelOpts.Text("Mode:", &lui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{255, 255, 255, 255},
		}),
	)
	modeRow.AddChild(modeLabel)

	lui.gameModeLabel = widget.NewLabel(
		widget.LabelOpts.Text(systems.GetGameModeName(lui.Lobby.GameMode), &lui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{255, 255, 100, 255},
		}),
	)
	modeRow.AddChild(lui.gameModeLabel)

	modeButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(50, 18)),
		widget.ButtonOpts.Image(lui.buttonImage()),
		widget.ButtonOpts.Text("Change", &lui.smallFace, &widget.ButtonTextColor{
			Idle:    color.RGBA{200, 200, 200, 255},
			Hover:   color.RGBA{255, 255, 255, 255},
			Pressed: color.RGBA{150, 150, 150, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			systems.CycleGameMode(lui.Lobby)
			lui.UpdateUI()
		}),
	)
	modeRow.AddChild(modeButton)

	container.AddChild(modeRow)

	// Match time row
	timeRow := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	timeLabel := widget.NewLabel(
		widget.LabelOpts.Text("Time:", &lui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{255, 255, 255, 255},
		}),
	)
	timeRow.AddChild(timeLabel)

	lui.matchTimeLabel = widget.NewLabel(
		widget.LabelOpts.Text(fmt.Sprintf("%d min", lui.Lobby.MatchMinutes), &lui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{255, 255, 100, 255},
		}),
	)
	timeRow.AddChild(lui.matchTimeLabel)

	timeButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(50, 18)),
		widget.ButtonOpts.Image(lui.buttonImage()),
		widget.ButtonOpts.Text("Change", &lui.smallFace, &widget.ButtonTextColor{
			Idle:    color.RGBA{200, 200, 200, 255},
			Hover:   color.RGBA{255, 255, 255, 255},
			Pressed: color.RGBA{150, 150, 150, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			systems.CycleMatchTime(lui.Lobby)
			lui.UpdateUI()
		}),
	)
	timeRow.AddChild(timeButton)

	container.AddChild(timeRow)

	return container
}

func (lui *LobbyUI) buildButtonsContainer() *widget.Container {
	container := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(10),
		)),
	)

	// Back button
	backButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(80, 28)),
		widget.ButtonOpts.Image(lui.buttonImage()),
		widget.ButtonOpts.Text("Back", &lui.normalFace, &widget.ButtonTextColor{
			Idle:    color.RGBA{255, 255, 255, 255},
			Hover:   color.RGBA{255, 200, 200, 255},
			Pressed: color.RGBA{200, 150, 150, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			if lui.OnGoBack != nil {
				lui.OnGoBack()
			}
		}),
	)
	container.AddChild(backButton)

	// Start button
	lui.startButton = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(100, 28)),
		widget.ButtonOpts.Image(lui.startButtonImage()),
		widget.ButtonOpts.Text("START", &lui.normalFace, &widget.ButtonTextColor{
			Idle:     color.RGBA{255, 255, 255, 255},
			Hover:    color.RGBA{200, 255, 200, 255},
			Pressed:  color.RGBA{150, 200, 150, 255},
			Disabled: color.RGBA{100, 100, 100, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			if systems.CanStartMatch(lui.Lobby) && lui.OnStartMatch != nil {
				lui.OnStartMatch()
			}
		}),
	)
	container.AddChild(lui.startButton)

	return container
}

func (lui *LobbyUI) buttonImage() *widget.ButtonImage {
	idle := image.NewNineSliceColor(color.RGBA{60, 60, 80, 255})
	hover := image.NewNineSliceColor(color.RGBA{80, 80, 100, 255})
	pressed := image.NewNineSliceColor(color.RGBA{40, 40, 60, 255})
	disabled := image.NewNineSliceColor(color.RGBA{40, 40, 40, 255})

	return &widget.ButtonImage{
		Idle:     idle,
		Hover:    hover,
		Pressed:  pressed,
		Disabled: disabled,
	}
}

func (lui *LobbyUI) startButtonImage() *widget.ButtonImage {
	idle := image.NewNineSliceColor(color.RGBA{40, 100, 40, 255})
	hover := image.NewNineSliceColor(color.RGBA{60, 140, 60, 255})
	pressed := image.NewNineSliceColor(color.RGBA{30, 80, 30, 255})
	disabled := image.NewNineSliceColor(color.RGBA{40, 50, 40, 255})

	return &widget.ButtonImage{
		Idle:     idle,
		Hover:    hover,
		Pressed:  pressed,
		Disabled: disabled,
	}
}

// UpdateUI updates all UI elements to reflect current lobby state
func (lui *LobbyUI) UpdateUI() {
	// Update gamepad detection
	systems.UpdateDetectedGamepads(lui.Lobby)

	// Update player slots
	for i := 0; i < 4; i++ {
		slot := &lui.Lobby.Slots[i]

		// Update type button text (check for nil to handle uninitialized widgets)
		if lui.slotTypeButtons[i] != nil {
			if textWidget := lui.slotTypeButtons[i].Text(); textWidget != nil {
				textWidget.Label = systems.GetSlotTypeName(slot.Type)
			}
		}

		// Update scheme button
		if lui.schemeButtons[i] != nil {
			if textWidget := lui.schemeButtons[i].Text(); textWidget != nil {
				textWidget.Label = systems.GetInputDeviceName(slot)
			}
			// Only enable scheme button for human keyboard players
			canCycleScheme := slot.Type == components.SlotHuman && slot.GamepadID == nil
			lui.schemeButtons[i].GetWidget().Disabled = !canCycleScheme
		}

		// Update difficulty label (only for bots)
		if lui.diffLabels[i] != nil {
			if slot.Type == components.SlotBot {
				lui.diffLabels[i].Label = systems.GetBotDifficultyName(slot.BotDifficulty)
			} else {
				lui.diffLabels[i].Label = ""
			}
		}

		// Update team label (only in team modes)
		if lui.teamLabels[i] != nil {
			if lui.Lobby.GameMode == cfg.GameMode2v2 && slot.Type != components.SlotEmpty {
				lui.teamLabels[i].Label = "Team: " + systems.GetTeamName(slot.Team)
			} else {
				lui.teamLabels[i].Label = ""
			}
		}
	}

	// Update settings
	if lui.gameModeLabel != nil {
		lui.gameModeLabel.Label = systems.GetGameModeName(lui.Lobby.GameMode)
	}
	if lui.matchTimeLabel != nil {
		lui.matchTimeLabel.Label = fmt.Sprintf("%d min", lui.Lobby.MatchMinutes)
	}

	// Update start button state
	if lui.startButton != nil {
		canStart := systems.CanStartMatch(lui.Lobby)
		lui.startButton.GetWidget().Disabled = !canStart

		// Update status message
		if lui.statusLabel != nil {
			if !canStart {
				lui.statusLabel.Label = lui.getValidationMessage()
			} else {
				lui.statusLabel.Label = ""
			}
		}
	}
}

func (lui *LobbyUI) getValidationMessage() string {
	humanCount := systems.GetHumanCount(lui.Lobby)
	playerCount := systems.GetActivePlayerCount(lui.Lobby)
	botCount := systems.GetBotCount(lui.Lobby)

	if humanCount < 1 {
		return "Need at least 1 human player"
	}
	if lui.Lobby.GameMode == cfg.GameMode1v1 && playerCount != 2 {
		return "1v1 requires exactly 2 players"
	}
	if lui.Lobby.GameMode == cfg.GameMode2v2 && playerCount != 4 {
		return "2v2 requires exactly 4 players"
	}
	if lui.Lobby.GameMode == cfg.GameModeCoopVsBots && botCount < 1 {
		return "Co-op mode requires at least 1 bot"
	}
	if playerCount < 2 {
		return "Need at least 2 players"
	}
	return ""
}

// Update calls the UI's Update method
func (lui *LobbyUI) Update() {
	lui.UI.Update()
}
