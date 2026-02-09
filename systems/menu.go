package systems

import (
	"os"

	"github.com/automoto/doomerang-mp/components"
	cfg "github.com/automoto/doomerang-mp/config"
	"github.com/automoto/doomerang-mp/fonts"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text" //nolint:staticcheck // TODO: migrate to text/v2
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/yohamta/donburi/ecs"
)

// SceneChanger allows systems to trigger scene transitions
type SceneChanger interface {
	ChangeScene(scene interface{})
}

// PlatformerSceneCreator creates a new platformer scene
type PlatformerSceneCreator interface {
	NewPlatformerScene() interface{}
}

// NewUpdateMenu creates an UpdateMenu system with scene transition capability
func NewUpdateMenu(sceneChanger SceneChanger, createPlatformerScene func() interface{}, createServerBrowserScene func() interface{}) ecs.System {
	return func(e *ecs.ECS) {
		// Skip menu input if settings is open
		if IsSettingsOpen(e) {
			return
		}

		menu := GetOrCreateMenu(e)
		input := getOrCreateInput(e)

		// Navigate menu with wrap-around
		numOptions := len(menu.VisibleOptions)
		if numOptions == 0 {
			return
		}

		if GetAction(input, cfg.ActionMenuUp).JustPressed {
			PlaySFX(e, cfg.SoundMenuNavigate)
			menu.SelectedIndex = (menu.SelectedIndex - 1 + numOptions) % numOptions
		}
		if GetAction(input, cfg.ActionMenuDown).JustPressed {
			PlaySFX(e, cfg.SoundMenuNavigate)
			menu.SelectedIndex = (menu.SelectedIndex + 1) % numOptions
		}

		// Handle selection
		if GetAction(input, cfg.ActionMenuSelect).JustPressed {
			PlaySFX(e, cfg.SoundMenuSelect)
			selectedOption := menu.VisibleOptions[menu.SelectedIndex]

			switch selectedOption {
			case components.MainMenuMultiplayer:
				FadeOutMusic(e)
				sceneChanger.ChangeScene(createServerBrowserScene())
			case components.MainMenuSettings:
				OpenSettings(e, false)
			case components.MainMenuExit:
				os.Exit(0)
			}
		}

		// Allow back/escape to exit
		if GetAction(input, cfg.ActionMenuBack).JustPressed {
			os.Exit(0)
		}
	}
}

// DrawMenu renders the main menu screen
func DrawMenu(e *ecs.ECS, screen *ebiten.Image) {
	menu := GetOrCreateMenu(e)

	width := float64(screen.Bounds().Dx())
	height := float64(screen.Bounds().Dy())

	// Draw background
	vector.FillRect(
		screen,
		0, 0,
		float32(width), float32(height),
		cfg.Menu.BackgroundColor,
		false,
	)

	// Draw title
	titleFont := fonts.ExcelTitle.Get()
	title := "DOOMERANG"
	titleWidth := len(title) * 20 // Approximate width for 32pt font
	titleX := int((width - float64(titleWidth)) / 2)
	text.Draw(screen, title, titleFont, titleX, int(cfg.Menu.TitleY), cfg.Menu.TitleColor)

	// Draw menu options
	menuFont := fonts.ExcelBold.Get()

	for i, option := range menu.VisibleOptions {
		y := cfg.Menu.MenuStartY + float64(i)*(cfg.Menu.MenuItemHeight+cfg.Menu.MenuItemGap)

		// Determine color based on selection
		textColor := cfg.Menu.TextColorNormal
		if i == menu.SelectedIndex {
			textColor = cfg.Menu.TextColorSelected
		}

		// Get option label
		label := getOptionLabel(option)
		textWidth := len(label) * 12
		x := int((width - float64(textWidth)) / 2)

		text.Draw(screen, label, menuFont, x, int(y)+int(cfg.Menu.MenuItemHeight), textColor)
	}

	// Draw navigation hint at bottom based on input method
	input := getOrCreateInput(e)
	hint := getMenuHint(input.LastInputMethod)
	hintFont := fonts.ExcelSmall.Get()
	hintWidth := len(hint) * 7
	hintX := int((width - float64(hintWidth)) / 2)
	text.Draw(screen, hint, hintFont, hintX, int(height)-12, cfg.Menu.TextColorNormal)
}

// getMenuHint returns the appropriate hint for menu navigation
func getMenuHint(method components.InputMethod) string {
	switch method {
	case components.InputPlayStation:
		return "Left Stick/D-Pad: Navigate   Cross: Select"
	case components.InputXbox:
		return "Left Stick/D-Pad: Navigate   A: Select"
	}
	return "Arrows: Navigate   Enter: Select"
}

// getOptionLabel returns the display text for a menu option
func getOptionLabel(option components.MainMenuOption) string {
	switch option {
	case components.MainMenuMultiplayer:
		return "Multiplayer"
	case components.MainMenuSettings:
		return "Settings"
	case components.MainMenuExit:
		return "Exit"
	default:
		return ""
	}
}

// GetOrCreateMenu returns the singleton Menu component, creating if needed
func GetOrCreateMenu(e *ecs.ECS) *components.MenuData {
	if _, ok := components.Menu.First(e.World); !ok {
		// Fixed menu options for multiplayer-only mode
		visibleOptions := []components.MainMenuOption{
			components.MainMenuMultiplayer,
			components.MainMenuSettings,
			components.MainMenuExit,
		}

		ent := e.World.Entry(e.World.Create(components.Menu))
		components.Menu.SetValue(ent, components.MenuData{
			SelectedIndex:  0,
			VisibleOptions: visibleOptions,
		})
	}

	ent, _ := components.Menu.First(e.World)
	return components.Menu.Get(ent)
}
