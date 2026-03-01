package components

import "github.com/yohamta/donburi"

type NetworkConfigData struct {
	IsNetwork bool
}

var NetworkConfig = donburi.NewComponentType[NetworkConfigData]()
