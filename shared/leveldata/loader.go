package leveldata

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lafriks/go-tiled"
)

// LoadCollisionData parses a TMX file and returns collision data (solid tiles
// and player spawn points). It takes an fs.FS so callers can pass embed.FS
// (client) or os.DirFS (server).
func LoadCollisionData(fsys fs.FS, tmxPath string) (*CollisionData, error) {
	levelMap, err := tiled.LoadFile(tmxPath, tiled.WithFileSystem(fsys))
	if err != nil {
		return nil, fmt.Errorf("load TMX %s: %w", tmxPath, err)
	}

	data := &CollisionData{
		MapWidth:  levelMap.Width * levelMap.TileWidth,
		MapHeight: levelMap.Height * levelMap.TileHeight,
	}

	// Parse solid tiles from wg-tiles layer
	tileW := float64(levelMap.TileWidth)
	tileH := float64(levelMap.TileHeight)
	for _, layer := range levelMap.Layers {
		if layer.Name != "wg-tiles" {
			continue
		}
		for y := 0; y < levelMap.Height; y++ {
			for x := 0; x < levelMap.Width; x++ {
				tile := layer.Tiles[y*levelMap.Width+x]
				if tile.IsNil() {
					continue
				}

				var slopeType string
				if tilesetTile, err := tile.Tileset.GetTilesetTile(tile.ID); err == nil {
					slopeType = tilesetTile.Properties.GetString("slope")
				}

				data.SolidRects = append(data.SolidRects, SolidRect{
					X:         float64(x) * tileW,
					Y:         float64(y) * tileH,
					W:         tileW,
					H:         tileH,
					SlopeType: slopeType,
				})
			}
		}
		break
	}

	// Parse player spawn points from PlayerSpawn object group
	for _, og := range levelMap.ObjectGroups {
		if og.Name != "PlayerSpawn" {
			continue
		}
		for _, o := range og.Objects {
			spawnIndex := o.Properties.GetInt("spawnIndex")
			data.SpawnPoints = append(data.SpawnPoints, SpawnPoint{
				X:     o.X,
				Y:     o.Y,
				Index: spawnIndex,
			})
		}
	}

	// Sort spawns left-to-right for consistent assignment
	sort.Slice(data.SpawnPoints, func(i, j int) bool {
		return data.SpawnPoints[i].X < data.SpawnPoints[j].X
	})

	return data, nil
}

// LoadAllLevels discovers all .tmx files in levelsDir within fsys, loads collision
// data for each, and returns a map keyed by stem name plus a sorted list of names.
func LoadAllLevels(fsys fs.FS, levelsDir string) (map[string]*CollisionData, []string, error) {
	pattern := levelsDir + "/*.tmx"
	matches, err := fs.Glob(fsys, pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("glob %s: %w", pattern, err)
	}
	if len(matches) == 0 {
		return nil, nil, fmt.Errorf("no .tmx files found in %s", levelsDir)
	}

	levels := make(map[string]*CollisionData, len(matches))
	names := make([]string, 0, len(matches))

	for _, path := range matches {
		data, err := LoadCollisionData(fsys, path)
		if err != nil {
			return nil, nil, fmt.Errorf("load %s: %w", path, err)
		}
		stem := strings.TrimSuffix(filepath.Base(path), ".tmx")
		levels[stem] = data
		names = append(names, stem)
	}

	sort.Strings(names)
	return levels, names, nil
}
