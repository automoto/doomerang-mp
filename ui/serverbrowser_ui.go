package ui

import (
	"bytes"
	"image/color"
	"log"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

type ServerBrowserUI struct {
	UI *ebitenui.UI

	OnConnect func(address string)
	OnGoBack  func()

	ipInput     *widget.TextInput
	portInput   *widget.TextInput
	statusLabel *widget.Label
	connectBtn  *widget.Button

	titleFace  text.Face
	normalFace text.Face
	smallFace  text.Face
}

func NewServerBrowserUI(onConnect func(address string), onGoBack func()) *ServerBrowserUI {
	ui := &ServerBrowserUI{
		OnConnect: onConnect,
		OnGoBack:  onGoBack,
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

	directConnectPanel := ui.buildDirectConnectPanel()
	contentContainer.AddChild(directConnectPanel)

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
		{"Browse", false},
		{"Direct Connect", true},
		{"Favorites", false},
		{"Recent", false},
	}

	for _, tab := range tabs {
		tabColor := color.RGBA{60, 60, 80, 255}
		textColor := color.RGBA{100, 100, 100, 255}
		if tab.enabled {
			tabColor = color.RGBA{80, 80, 120, 255}
			textColor = color.RGBA{255, 255, 255, 255}
		}

		tabBtn := widget.NewButton(
			widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(80, 22)),
			widget.ButtonOpts.Image(&widget.ButtonImage{
				Idle:     image.NewNineSliceColor(tabColor),
				Hover:    image.NewNineSliceColor(tabColor),
				Pressed:  image.NewNineSliceColor(tabColor),
				Disabled: image.NewNineSliceColor(color.RGBA{40, 40, 40, 255}),
			}),
			widget.ButtonOpts.Text(tab.label, &ui.smallFace, &widget.ButtonTextColor{
				Idle:     textColor,
				Disabled: color.RGBA{80, 80, 80, 255},
			}),
		)
		if !tab.enabled {
			tabBtn.GetWidget().Disabled = true
		}
		container.AddChild(tabBtn)
	}

	return container
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
				ui.OnConnect(addr)
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

func (ui *ServerBrowserUI) Update() {
	ui.UI.Update()
}
