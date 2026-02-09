package components

import "github.com/yohamta/donburi"

// MainMenuOption represents the available main menu selections
type MainMenuOption int

const (
	MainMenuLocalPlay MainMenuOption = iota
	MainMenuMultiplayer
	MainMenuSettings
	MainMenuExit
)

// MenuData stores the current state of the main menu
type MenuData struct {
	SelectedIndex  int              // Current selection index in VisibleOptions
	VisibleOptions []MainMenuOption // Options to display
}

// Menu is the component type for main menu state
var Menu = donburi.NewComponentType[MenuData]()
