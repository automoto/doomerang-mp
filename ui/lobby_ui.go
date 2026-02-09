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
	diffButtons     [4]*widget.Button // Bot difficulty buttons
	teamButtons     [4]*widget.Button // Team selection buttons
	gameModeLabel   *widget.Label
	matchTimeLabel  *widget.Label
	levelLabel      *widget.Label
	startButton     *widget.Button
	statusLabel     *widget.Label

	// Fonts (stored as interface for ebitenui compatibility)
	titleFace  text.Face
	normalFace text.Face
	smallFace  text.Face

	// Initialization tracking
	initialized bool
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
			Idle: cfg.PlayerColors.Colors[slotIndex].RGBA,
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

	// Difficulty button (for bots) - clickable to change difficulty
	initialDiffLabel := ""
	if slot.Type == components.SlotBot {
		initialDiffLabel = systems.GetBotDifficultyName(slot.BotDifficulty)
	}
	diffButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(55, 20),
		),
		widget.ButtonOpts.Image(lui.buttonImage()),
		widget.ButtonOpts.Text(initialDiffLabel, &lui.smallFace, &widget.ButtonTextColor{
			Idle:     color.RGBA{180, 180, 180, 255},
			Hover:    color.RGBA{255, 255, 200, 255},
			Pressed:  color.RGBA{200, 200, 200, 255},
			Disabled: color.RGBA{100, 100, 100, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			systems.CycleBotDifficulty(lui.Lobby, idx)
			lui.UpdateUI()
		}),
	)
	lui.diffButtons[slotIndex] = diffButton
	row.AddChild(diffButton)

	// Team button (for team modes) - clickable to change team
	teamButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(55, 20),
		),
		widget.ButtonOpts.Image(lui.buttonImage()),
		widget.ButtonOpts.Text("", &lui.smallFace, &widget.ButtonTextColor{
			Idle:     color.RGBA{180, 180, 180, 255},
			Hover:    color.RGBA{255, 255, 200, 255},
			Pressed:  color.RGBA{200, 200, 200, 255},
			Disabled: color.RGBA{100, 100, 100, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			systems.CycleTeam(lui.Lobby, idx)
			lui.UpdateUI()
		}),
	)
	lui.teamButtons[slotIndex] = teamButton
	row.AddChild(teamButton)

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

	// Level row
	if len(lui.Lobby.LevelNames) > 0 {
		levelRow := widget.NewContainer(
			widget.ContainerOpts.Layout(widget.NewRowLayout(
				widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
				widget.RowLayoutOpts.Spacing(6),
			)),
		)

		levelTitleLabel := widget.NewLabel(
			widget.LabelOpts.Text("Level:", &lui.smallFace, &widget.LabelColor{
				Idle: color.RGBA{255, 255, 255, 255},
			}),
		)
		levelRow.AddChild(levelTitleLabel)

		displayName := systems.GetLevelDisplayName(lui.Lobby.LevelNames[lui.Lobby.LevelIndex])
		lui.levelLabel = widget.NewLabel(
			widget.LabelOpts.Text(displayName, &lui.smallFace, &widget.LabelColor{
				Idle: color.RGBA{255, 255, 100, 255},
			}),
		)
		levelRow.AddChild(lui.levelLabel)

		levelButton := widget.NewButton(
			widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(50, 18)),
			widget.ButtonOpts.Image(lui.buttonImage()),
			widget.ButtonOpts.Text("Change", &lui.smallFace, &widget.ButtonTextColor{
				Idle:    color.RGBA{200, 200, 200, 255},
				Hover:   color.RGBA{255, 255, 255, 255},
				Pressed: color.RGBA{150, 150, 150, 255},
			}),
			widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
				systems.CycleLevel(lui.Lobby)
				lui.UpdateUI()
			}),
		)
		levelRow.AddChild(levelButton)

		container.AddChild(levelRow)
	}

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

		// Update difficulty button (only for bots)
		if lui.diffButtons[i] != nil {
			isBot := slot.Type == components.SlotBot
			if textWidget := lui.diffButtons[i].Text(); textWidget != nil {
				if isBot {
					textWidget.Label = systems.GetBotDifficultyName(slot.BotDifficulty)
				} else {
					textWidget.Label = ""
				}
			}
			lui.diffButtons[i].GetWidget().Disabled = !isBot
		}

		// Update team button (only in team modes)
		if lui.teamButtons[i] != nil {
			isTeamMode := lui.Lobby.GameMode == cfg.GameMode2v2 || lui.Lobby.GameMode == cfg.GameModeCoopVsBots
			isActive := slot.Type != components.SlotEmpty
			showTeam := isTeamMode && isActive
			if textWidget := lui.teamButtons[i].Text(); textWidget != nil {
				if showTeam {
					textWidget.Label = systems.GetTeamName(slot.Team)
				} else {
					textWidget.Label = ""
				}
			}
			// Allow manual team changes in both 2v2 and Co-op modes
			canChangeTeam := isTeamMode && isActive
			lui.teamButtons[i].GetWidget().Disabled = !canChangeTeam
		}
	}

	// Update settings
	if lui.gameModeLabel != nil {
		lui.gameModeLabel.Label = systems.GetGameModeName(lui.Lobby.GameMode)
	}
	if lui.matchTimeLabel != nil {
		lui.matchTimeLabel.Label = fmt.Sprintf("%d min", lui.Lobby.MatchMinutes)
	}
	if lui.levelLabel != nil && len(lui.Lobby.LevelNames) > 0 {
		lui.levelLabel.Label = systems.GetLevelDisplayName(lui.Lobby.LevelNames[lui.Lobby.LevelIndex])
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
	if lui.Lobby.GameMode == cfg.GameModeCoopVsBots {
		if botCount < 1 {
			return "Co-op mode requires at least 1 bot"
		}
		// Check team balance
		if !lui.hasPlayersOnBothTeams() {
			return "Need at least 1 player on each team"
		}
	}
	if playerCount < 2 {
		return "Need at least 2 players"
	}
	return ""
}

func (lui *LobbyUI) hasPlayersOnBothTeams() bool {
	team0, team1 := 0, 0
	for _, slot := range lui.Lobby.Slots {
		if slot.Type == components.SlotEmpty {
			continue
		}
		switch slot.Team {
		case 0:
			team0++
		case 1:
			team1++
		}
	}
	return team0 >= 1 && team1 >= 1
}

// Update calls the UI's Update method
func (lui *LobbyUI) Update() {
	lui.UI.Update()
	// Update UI state on first frame after widgets are validated
	if !lui.initialized {
		lui.initialized = true
		lui.UpdateUI()
	}
}
