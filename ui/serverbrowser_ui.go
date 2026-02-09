package ui

import (
	"bytes"
	"fmt"
	"image/color"
	"log"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

// ServerEntry represents a game server returned from the master server.
type ServerEntry struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	Players    int    `json:"players"`
	MaxPlayers int    `json:"maxPlayers"`
	Version    string `json:"version"`
}

type ServerBrowserUI struct {
	UI *ebitenui.UI

	OnConnect func(address, level string)
	OnGoBack  func()
	OnRefresh func()

	ipInput     *widget.TextInput
	portInput   *widget.TextInput
	statusLabel *widget.Label
	connectBtn  *widget.Button

	levelNames      []string
	selectedLevelIdx int
	levelLabel       *widget.Label

	activeTab           int
	browsePanel         *widget.Container
	directPanel         *widget.Container
	panelParent         *widget.Container
	serverListContainer *widget.Container
	refreshBtn          *widget.Button
	browseStatusLabel   *widget.Label
	tabButtons          []*widget.Button
	tabActiveImage      *widget.ButtonImage
	tabInactiveImage    *widget.ButtonImage

	titleFace  text.Face
	normalFace text.Face
	smallFace  text.Face
}

func NewServerBrowserUI(onConnect func(address, level string), onGoBack func(), onRefresh func(), levelNames []string) *ServerBrowserUI {
	ui := &ServerBrowserUI{
		OnConnect:  onConnect,
		OnGoBack:   onGoBack,
		OnRefresh:  onRefresh,
		levelNames: levelNames,
	}
	ui.loadFonts()
	ui.buildUI()
	return ui
}

func (ui *ServerBrowserUI) loadFonts() {
	fontSource, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		log.Fatalf("failed to load UI font: %v", err)
	}

	ui.titleFace = &text.GoTextFace{Source: fontSource, Size: 18}
	ui.normalFace = &text.GoTextFace{Source: fontSource, Size: 12}
	ui.smallFace = &text.GoTextFace{Source: fontSource, Size: 10}
}

func (ui *ServerBrowserUI) buildUI() {
	rootContainer := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.RGBA{20, 20, 30, 255})),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)

	contentContainer := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(12)),
			widget.RowLayoutOpts.Spacing(8),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
			}),
		),
	)

	titleLabel := widget.NewLabel(
		widget.LabelOpts.Text("MULTIPLAYER", &ui.titleFace, &widget.LabelColor{
			Idle: color.RGBA{255, 255, 255, 255},
		}),
	)
	contentContainer.AddChild(titleLabel)

	tabContainer := ui.buildTabBar()
	contentContainer.AddChild(tabContainer)

	ui.browsePanel = ui.buildBrowsePanel()
	ui.directPanel = ui.buildDirectConnectPanel()

	ui.panelParent = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewStackedLayout()),
	)
	// Default to Browse tab
	ui.panelParent.AddChild(ui.browsePanel)
	ui.activeTab = 0
	contentContainer.AddChild(ui.panelParent)

	ui.statusLabel = widget.NewLabel(
		widget.LabelOpts.Text("", &ui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{255, 200, 100, 255},
		}),
	)
	contentContainer.AddChild(ui.statusLabel)

	buttonsContainer := ui.buildButtons()
	contentContainer.AddChild(buttonsContainer)

	rootContainer.AddChild(contentContainer)

	ui.UI = &ebitenui.UI{Container: rootContainer}
}

func (ui *ServerBrowserUI) buildTabBar() *widget.Container {
	container := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(4),
		)),
	)

	tabs := []struct {
		label   string
		enabled bool
	}{
		{"Browse", true},
		{"Direct Connect", true},
		{"Favorites", false},
		{"Recent", false},
	}

	ui.tabButtons = nil

	activeColor := color.RGBA{80, 80, 120, 255}
	inactiveColor := color.RGBA{60, 60, 80, 255}
	disabledColor := color.RGBA{40, 40, 40, 255}

	ui.tabActiveImage = &widget.ButtonImage{
		Idle:     image.NewNineSliceColor(activeColor),
		Hover:    image.NewNineSliceColor(activeColor),
		Pressed:  image.NewNineSliceColor(activeColor),
		Disabled: image.NewNineSliceColor(disabledColor),
	}
	ui.tabInactiveImage = &widget.ButtonImage{
		Idle:     image.NewNineSliceColor(inactiveColor),
		Hover:    image.NewNineSliceColor(inactiveColor),
		Pressed:  image.NewNineSliceColor(activeColor),
		Disabled: image.NewNineSliceColor(disabledColor),
	}

	for i, tab := range tabs {
		idx := i
		startImage := ui.tabInactiveImage
		startTextColor := color.RGBA{180, 180, 180, 255}
		if idx == 0 {
			startImage = ui.tabActiveImage
			startTextColor = color.RGBA{255, 255, 255, 255}
		}

		tabBtn := widget.NewButton(
			widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(80, 22)),
			widget.ButtonOpts.Image(startImage),
			widget.ButtonOpts.Text(tab.label, &ui.smallFace, &widget.ButtonTextColor{
				Idle:     startTextColor,
				Disabled: color.RGBA{80, 80, 80, 255},
			}),
			widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
				ui.switchTab(idx)
			}),
		)
		if !tab.enabled {
			tabBtn.GetWidget().Disabled = true
		}
		container.AddChild(tabBtn)
		ui.tabButtons = append(ui.tabButtons, tabBtn)
	}

	return container
}

func (ui *ServerBrowserUI) switchTab(idx int) {
	if idx == ui.activeTab {
		return
	}

	ui.panelParent.RemoveChildren()

	for i, btn := range ui.tabButtons {
		if btn.GetWidget().Disabled {
			continue
		}
		if i == idx {
			btn.SetImage(ui.tabActiveImage)
		} else {
			btn.SetImage(ui.tabInactiveImage)
		}
	}

	switch idx {
	case 0:
		ui.panelParent.AddChild(ui.browsePanel)
	case 1:
		ui.panelParent.AddChild(ui.directPanel)
	}

	ui.activeTab = idx
}

func (ui *ServerBrowserUI) buildBrowsePanel() *widget.Container {
	padding := widget.Insets{Top: 6, Bottom: 6, Left: 8, Right: 8}
	panel := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.RGBA{30, 30, 45, 255})),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(&padding),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	// Refresh button row
	topRow := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
		)),
	)

	ui.refreshBtn = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(80, 22)),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:     image.NewNineSliceColor(color.RGBA{40, 80, 120, 255}),
			Hover:    image.NewNineSliceColor(color.RGBA{60, 100, 140, 255}),
			Pressed:  image.NewNineSliceColor(color.RGBA{30, 60, 100, 255}),
			Disabled: image.NewNineSliceColor(color.RGBA{40, 50, 60, 255}),
		}),
		widget.ButtonOpts.Text("Refresh", &ui.normalFace, &widget.ButtonTextColor{
			Idle:     color.RGBA{255, 255, 255, 255},
			Disabled: color.RGBA{100, 100, 100, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			if ui.OnRefresh != nil {
				ui.OnRefresh()
			}
		}),
	)
	topRow.AddChild(ui.refreshBtn)

	ui.browseStatusLabel = widget.NewLabel(
		widget.LabelOpts.Text("", &ui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{180, 180, 180, 255},
		}),
	)
	topRow.AddChild(ui.browseStatusLabel)

	panel.AddChild(topRow)

	// Column headers
	headerRow := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
		)),
	)
	headerRow.AddChild(widget.NewLabel(
		widget.LabelOpts.Text("Server Name", &ui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{150, 150, 150, 255},
		}),
		widget.LabelOpts.TextOpts(widget.TextOpts.WidgetOpts(widget.WidgetOpts.MinSize(180, 0))),
	))
	headerRow.AddChild(widget.NewLabel(
		widget.LabelOpts.Text("Players", &ui.smallFace, &widget.LabelColor{
			Idle: color.RGBA{150, 150, 150, 255},
		}),
	))
	panel.AddChild(headerRow)

	// Server list (scrollable area)
	ui.serverListContainer = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(2),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(300, 80)),
	)
	panel.AddChild(ui.serverListContainer)

	return panel
}

func (ui *ServerBrowserUI) SetServerList(servers []ServerEntry) {
	ui.serverListContainer.RemoveChildren()

	if len(servers) == 0 {
		ui.serverListContainer.AddChild(widget.NewLabel(
			widget.LabelOpts.Text("No servers found", &ui.smallFace, &widget.LabelColor{
				Idle: color.RGBA{120, 120, 120, 255},
			}),
		))
		return
	}

	for _, srv := range servers {
		addr := srv.Address
		row := widget.NewContainer(
			widget.ContainerOpts.Layout(widget.NewRowLayout(
				widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
				widget.RowLayoutOpts.Spacing(8),
			)),
		)

		row.AddChild(widget.NewLabel(
			widget.LabelOpts.Text(srv.Name, &ui.smallFace, &widget.LabelColor{
				Idle: color.RGBA{255, 255, 255, 255},
			}),
			widget.LabelOpts.TextOpts(widget.TextOpts.WidgetOpts(widget.WidgetOpts.MinSize(180, 0))),
		))

		row.AddChild(widget.NewLabel(
			widget.LabelOpts.Text(fmt.Sprintf("%d/%d", srv.Players, srv.MaxPlayers), &ui.smallFace, &widget.LabelColor{
				Idle: color.RGBA{200, 200, 200, 255},
			}),
			widget.LabelOpts.TextOpts(widget.TextOpts.WidgetOpts(widget.WidgetOpts.MinSize(40, 0))),
		))

		joinBtn := widget.NewButton(
			widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(50, 20)),
			widget.ButtonOpts.Image(&widget.ButtonImage{
				Idle:    image.NewNineSliceColor(color.RGBA{40, 100, 40, 255}),
				Hover:   image.NewNineSliceColor(color.RGBA{60, 140, 60, 255}),
				Pressed: image.NewNineSliceColor(color.RGBA{30, 80, 30, 255}),
			}),
			widget.ButtonOpts.Text("Join", &ui.smallFace, &widget.ButtonTextColor{
				Idle:    color.RGBA{255, 255, 255, 255},
				Hover:   color.RGBA{200, 255, 200, 255},
				Pressed: color.RGBA{150, 200, 150, 255},
			}),
			widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
				if ui.OnConnect != nil {
					ui.OnConnect(addr, "")
				}
			}),
		)
		row.AddChild(joinBtn)

		ui.serverListContainer.AddChild(row)
	}
}

func (ui *ServerBrowserUI) SetBrowseStatus(msg string) {
	if ui.browseStatusLabel != nil {
		ui.browseStatusLabel.Label = msg
	}
}

func (ui *ServerBrowserUI) SetRefreshing(refreshing bool) {
	if ui.refreshBtn != nil {
		ui.refreshBtn.GetWidget().Disabled = refreshing
	}
}

func (ui *ServerBrowserUI) buildDirectConnectPanel() *widget.Container {
	padding := widget.Insets{Top: 6, Bottom: 6, Left: 8, Right: 8}
	panel := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.RGBA{30, 30, 45, 255})),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(&padding),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	ipRow := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	ipLabel := widget.NewLabel(
		widget.LabelOpts.Text("Address:", &ui.normalFace, &widget.LabelColor{
			Idle: color.RGBA{200, 200, 200, 255},
		}),
	)
	ipRow.AddChild(ipLabel)

	ui.ipInput = widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(widget.WidgetOpts.MinSize(160, 22)),
		widget.TextInputOpts.Image(&widget.TextInputImage{
			Idle:     image.NewNineSliceColor(color.RGBA{50, 50, 70, 255}),
			Disabled: image.NewNineSliceColor(color.RGBA{40, 40, 50, 255}),
		}),
		widget.TextInputOpts.Face(&ui.normalFace),
		widget.TextInputOpts.Color(&widget.TextInputColor{
			Idle:          color.RGBA{255, 255, 255, 255},
			Disabled:      color.RGBA{128, 128, 128, 255},
			Caret:         color.RGBA{255, 255, 255, 255},
			DisabledCaret: color.RGBA{128, 128, 128, 255},
		}),
		widget.TextInputOpts.Placeholder("localhost"),
		widget.TextInputOpts.Padding(widget.NewInsetsSimple(4)),
	)
	ipRow.AddChild(ui.ipInput)

	panel.AddChild(ipRow)

	portRow := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	portLabel := widget.NewLabel(
		widget.LabelOpts.Text("Port:      ", &ui.normalFace, &widget.LabelColor{
			Idle: color.RGBA{200, 200, 200, 255},
		}),
	)
	portRow.AddChild(portLabel)

	ui.portInput = widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(widget.WidgetOpts.MinSize(80, 22)),
		widget.TextInputOpts.Image(&widget.TextInputImage{
			Idle:     image.NewNineSliceColor(color.RGBA{50, 50, 70, 255}),
			Disabled: image.NewNineSliceColor(color.RGBA{40, 40, 50, 255}),
		}),
		widget.TextInputOpts.Face(&ui.normalFace),
		widget.TextInputOpts.Color(&widget.TextInputColor{
			Idle:          color.RGBA{255, 255, 255, 255},
			Disabled:      color.RGBA{128, 128, 128, 255},
			Caret:         color.RGBA{255, 255, 255, 255},
			DisabledCaret: color.RGBA{128, 128, 128, 255},
		}),
		widget.TextInputOpts.Placeholder("7373"),
		widget.TextInputOpts.Padding(widget.NewInsetsSimple(4)),
	)
	portRow.AddChild(ui.portInput)

	panel.AddChild(portRow)

	// Level row
	if len(ui.levelNames) > 0 {
		levelRow := widget.NewContainer(
			widget.ContainerOpts.Layout(widget.NewRowLayout(
				widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
				widget.RowLayoutOpts.Spacing(6),
			)),
		)

		levelRowLabel := widget.NewLabel(
			widget.LabelOpts.Text("Level:    ", &ui.normalFace, &widget.LabelColor{
				Idle: color.RGBA{200, 200, 200, 255},
			}),
		)
		levelRow.AddChild(levelRowLabel)

		ui.levelLabel = widget.NewLabel(
			widget.LabelOpts.Text(ui.levelNames[0], &ui.normalFace, &widget.LabelColor{
				Idle: color.RGBA{255, 255, 100, 255},
			}),
			widget.LabelOpts.TextOpts(widget.TextOpts.WidgetOpts(widget.WidgetOpts.MinSize(100, 0))),
		)
		levelRow.AddChild(ui.levelLabel)

		levelBtn := widget.NewButton(
			widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(50, 22)),
			widget.ButtonOpts.Image(&widget.ButtonImage{
				Idle:    image.NewNineSliceColor(color.RGBA{60, 60, 80, 255}),
				Hover:   image.NewNineSliceColor(color.RGBA{80, 80, 100, 255}),
				Pressed: image.NewNineSliceColor(color.RGBA{40, 40, 60, 255}),
			}),
			widget.ButtonOpts.Text("Change", &ui.smallFace, &widget.ButtonTextColor{
				Idle:    color.RGBA{200, 200, 200, 255},
				Hover:   color.RGBA{255, 255, 255, 255},
				Pressed: color.RGBA{150, 150, 150, 255},
			}),
			widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
				ui.cycleLevel()
			}),
		)
		levelRow.AddChild(levelBtn)

		panel.AddChild(levelRow)
	}

	ui.connectBtn = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(120, 26)),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:     image.NewNineSliceColor(color.RGBA{40, 100, 40, 255}),
			Hover:    image.NewNineSliceColor(color.RGBA{60, 140, 60, 255}),
			Pressed:  image.NewNineSliceColor(color.RGBA{30, 80, 30, 255}),
			Disabled: image.NewNineSliceColor(color.RGBA{40, 50, 40, 255}),
		}),
		widget.ButtonOpts.Text("Connect", &ui.normalFace, &widget.ButtonTextColor{
			Idle:     color.RGBA{255, 255, 255, 255},
			Hover:    color.RGBA{200, 255, 200, 255},
			Pressed:  color.RGBA{150, 200, 150, 255},
			Disabled: color.RGBA{100, 100, 100, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			if ui.OnConnect != nil {
				addr := ui.getAddress()
				ui.OnConnect(addr, ui.SelectedLevel())
			}
		}),
	)
	panel.AddChild(ui.connectBtn)

	return panel
}

func (ui *ServerBrowserUI) buildButtons() *widget.Container {
	container := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(10),
		)),
	)

	backButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(80, 28)),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    image.NewNineSliceColor(color.RGBA{60, 60, 80, 255}),
			Hover:   image.NewNineSliceColor(color.RGBA{80, 80, 100, 255}),
			Pressed: image.NewNineSliceColor(color.RGBA{40, 40, 60, 255}),
		}),
		widget.ButtonOpts.Text("Back", &ui.normalFace, &widget.ButtonTextColor{
			Idle:    color.RGBA{255, 255, 255, 255},
			Hover:   color.RGBA{255, 200, 200, 255},
			Pressed: color.RGBA{200, 150, 150, 255},
		}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			if ui.OnGoBack != nil {
				ui.OnGoBack()
			}
		}),
	)
	container.AddChild(backButton)

	return container
}

func (ui *ServerBrowserUI) getAddress() string {
	host := ui.ipInput.GetText()
	if host == "" {
		host = "localhost"
	}
	port := ui.portInput.GetText()
	if port == "" {
		port = "7373"
	}
	return host + ":" + port
}

func (ui *ServerBrowserUI) SetStatus(msg string) {
	if ui.statusLabel != nil {
		ui.statusLabel.Label = msg
	}
}

func (ui *ServerBrowserUI) SetConnecting(connecting bool) {
	if ui.connectBtn != nil {
		ui.connectBtn.GetWidget().Disabled = connecting
	}
}

func (ui *ServerBrowserUI) cycleLevel() {
	if len(ui.levelNames) == 0 {
		return
	}
	ui.selectedLevelIdx = (ui.selectedLevelIdx + 1) % len(ui.levelNames)
	if ui.levelLabel != nil {
		ui.levelLabel.Label = ui.levelNames[ui.selectedLevelIdx]
	}
}

// SelectedLevel returns the currently selected level name, or empty if none.
func (ui *ServerBrowserUI) SelectedLevel() string {
	if len(ui.levelNames) == 0 {
		return ""
	}
	return ui.levelNames[ui.selectedLevelIdx]
}

func (ui *ServerBrowserUI) Update() {
	ui.UI.Update()
}
