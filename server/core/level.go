package core

import (
	"fmt"
	"log"
	"os"

	"github.com/automoto/doomerang-mp/shared/leveldata"
	"github.com/solarlune/resolv"
)

// ServerLevel holds the server's collision space and spawn data for a level.
type ServerLevel struct {
	Space       *resolv.Space
	SpawnPoints []leveldata.SpawnPoint
	MapWidth    int
	MapHeight   int
}

// NewServerLevel builds a resolv.Space from parsed collision data.
func NewServerLevel(data *leveldata.CollisionData) *ServerLevel {
	space := resolv.NewSpace(data.MapWidth, data.MapHeight, 16, 16)

	for _, r := range data.SolidRects {
		var obj *resolv.Object
		switch r.SlopeType {
		case tagSlope45UpR:
			obj = resolv.NewObject(r.X, r.Y, r.W, r.H, tagRamp, tagSlope45UpR)
		case tagSlope45UpL:
			obj = resolv.NewObject(r.X, r.Y, r.W, r.H, tagRamp, tagSlope45UpL)
		default:
			obj = resolv.NewObject(r.X, r.Y, r.W, r.H, tagSolid)
		}
		obj.SetShape(resolv.NewRectangle(0, 0, r.W, r.H))
		space.Add(obj)
	}

	log.Printf("Loaded level: %d solid tiles, %d spawn points, %dx%d map",
		len(data.SolidRects), len(data.SpawnPoints), data.MapWidth, data.MapHeight)

	return &ServerLevel{
		Space:       space,
		SpawnPoints: data.SpawnPoints,
		MapWidth:    data.MapWidth,
		MapHeight:   data.MapHeight,
	}
}

// LoadAllServerLevels loads all .tmx levels from the given assets directory,
// returning a map of ServerLevel keyed by stem name plus a sorted name list.
func LoadAllServerLevels(assetsDir string) (map[string]*ServerLevel, []string, error) {
	collisionMap, names, err := leveldata.LoadAllLevels(os.DirFS(assetsDir), "levels")
	if err != nil {
		return nil, nil, fmt.Errorf("load all levels: %w", err)
	}

	levels := make(map[string]*ServerLevel, len(names))
	for _, name := range names {
		levels[name] = NewServerLevel(collisionMap[name])
	}

	return levels, names, nil
}
