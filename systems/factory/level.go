package factory

import (
	"github.com/automoto/doomerang-mp/archetypes"
	"github.com/automoto/doomerang-mp/assets"
	"github.com/automoto/doomerang-mp/components"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

func CreateLevel(ecs *ecs.ECS) *donburi.Entry {
	return CreateLevelAtIndex(ecs, 0)
}

func CreateLevelAtIndex(ecs *ecs.ECS, levelIndex int) *donburi.Entry {
	level := archetypes.Level.Spawn(ecs)

	// Load all levels
	loader := assets.NewLevelLoader()
	levels := loader.MustLoadLevels()

	if len(levels) == 0 {
		panic("No levels found in assets/levels directory")
	}

	// Clamp index to valid range
	if levelIndex < 0 || levelIndex >= len(levels) {
		levelIndex = 0
	}

	// Set up level data
	levelData := &components.LevelData{
		Levels:       levels,
		LevelIndex:   levelIndex,
		CurrentLevel: &levels[levelIndex],
	}

	components.Level.Set(level, levelData)

	return level
}
