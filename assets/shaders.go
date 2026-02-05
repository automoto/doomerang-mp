package assets

import (
	"embed"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed shaders/*.kage
var shaderFS embed.FS

var (
	// TintShader is used to colorize player sprites
	TintShader *ebiten.Shader
)

// LoadShaders compiles and caches all shaders
func LoadShaders() error {
	var err error

	// Load tint shader
	tintSrc, err := shaderFS.ReadFile("shaders/tint.kage")
	if err != nil {
		return err
	}
	TintShader, err = ebiten.NewShader(tintSrc)
	if err != nil {
		return err
	}

	return nil
}
