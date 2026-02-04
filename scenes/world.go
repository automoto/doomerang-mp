package scenes

import (
	"errors"
	"image/color"
	"sync"

	"github.com/automoto/doomerang-mp/assets"
	cfg "github.com/automoto/doomerang-mp/config"
	factory2 "github.com/automoto/doomerang-mp/systems/factory"

	"github.com/automoto/doomerang-mp/components"
	"github.com/automoto/doomerang-mp/systems"
	"github.com/automoto/doomerang-mp/tags"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/ecs"
)

type PlatformerScene struct {
	ecs          *ecs.ECS
	sceneChanger SceneChanger
	once         sync.Once
}

// NewPlatformerScene creates a new platformer scene
func NewPlatformerScene(sc SceneChanger) *PlatformerScene {
	return &PlatformerScene{sceneChanger: sc}
}

func (ps *PlatformerScene) Update() {
	ps.once.Do(ps.configure)
	ps.ecs.Update()

	// Check for game over (player has 0 lives)
	if ps.checkGameOver() {
		ps.sceneChanger.ChangeScene(NewGameOverScene(ps.sceneChanger))
	}
}

// checkGameOver returns true if all player entities have been removed (after death sequence completes)
func (ps *PlatformerScene) checkGameOver() bool {
	if ps.ecs == nil {
		return false
	}

	// Game over when no players remain
	playerCount := 0
	tags.Player.Each(ps.ecs.World, func(entry *donburi.Entry) {
		playerCount++
	})
	return playerCount == 0
}

func (ps *PlatformerScene) Draw(screen *ebiten.Image) {
	// Always clear screen to prevent white flashes from OS window background
	screen.Fill(color.Black)

	if ps.ecs == nil {
		return
	}
	ps.ecs.Draw(screen)
}

func (ps *PlatformerScene) configure() {
	// Preload assets to avoid lag on first use (important for WASM)
	systems.PreloadAllSFX()
	assets.PreloadAllAnimations()

	ecs := ecs.NewECS(donburi.NewWorld())

	// Audio system (runs first, even when paused for menu sounds)
	ecs.AddSystem(systems.UpdateAudio)

	// Systems that always run
	ecs.AddSystem(systems.UpdateInput)
	ecs.AddSystem(systems.UpdateMultiPlayerInput)
	ecs.AddSystem(systems.UpdatePause)

	// Game systems wrapped with pause and level complete checks
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdatePlayer))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateEnemies))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateStates))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdatePhysics))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateCollisions))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateObjects))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateBoomerang))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateKnives))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateCombat))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateCombatHitboxes))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateDeaths))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateFire))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateEffects))
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateMessage))

	// Systems that run even when paused
	ecs.AddSystem(systems.UpdateSettings)
	ecs.AddSystem(systems.UpdateSettingsMenu)
	ecs.AddSystem(systems.WithGameplayChecks(systems.UpdateCamera))

	// Add renderers
	ecs.AddRenderer(cfg.Default, systems.DrawLevel)
	ecs.AddRenderer(cfg.Default, systems.DrawAnimated)
	ecs.AddRenderer(cfg.Default, systems.DrawSprites)
	ecs.AddRenderer(cfg.Default, systems.DrawHealthBars)
	ecs.AddRenderer(cfg.Default, systems.DrawHitboxes)
	ecs.AddRenderer(cfg.Default, systems.DrawHUD)
	ecs.AddRenderer(cfg.Default, systems.DrawMessage)
	ecs.AddRenderer(cfg.Default, systems.DrawDebug)
	ecs.AddRenderer(cfg.Default, systems.DrawPause)
	ecs.AddRenderer(cfg.Default, systems.DrawSettingsMenu)

	ps.ecs = ecs

	// Create the level entity and load level data FIRST.
	level := factory2.CreateLevel(ps.ecs)
	levelData := components.Level.Get(level)

	// Now create the space for collision detection using the level's dimensions.
	spaceEntry := factory2.CreateSpace(ps.ecs,
		levelData.CurrentLevel.Width,
		levelData.CurrentLevel.Height,
		16, 16,
	)
	space := components.Space.Get(spaceEntry)

	// Create camera
	factory2.CreateCamera(ps.ecs)

	// Create collision objects from solid tiles
	for _, tile := range levelData.CurrentLevel.SolidTiles {
		if tile.SlopeType != "" {
			factory2.CreateSlopeWall(ps.ecs, tile.X, tile.Y, tile.Width, tile.Height, tile.SlopeType)
		} else {
			factory2.CreateWall(ps.ecs, tile.X, tile.Y, tile.Width, tile.Height)
		}
	}

	// Create dead zones from the level
	for _, dz := range levelData.CurrentLevel.DeadZones {
		factory2.CreateDeadZone(ps.ecs, dz.X, dz.Y, dz.Width, dz.Height)
	}

	// Create fire obstacles from the level
	for _, fire := range levelData.CurrentLevel.Fires {
		factory2.CreateFire(ps.ecs, fire.X, fire.Y, fire.FireType, fire.Direction)
	}

	// Create message points from the level
	for _, msg := range levelData.CurrentLevel.Messages {
		factory2.CreateMessagePoint(ps.ecs, msg.X, msg.Y, msg.MessageID)
	}

	// Spawn players at available spawn points
	if len(levelData.CurrentLevel.PlayerSpawns) <= 0 {
		err := errors.New("no player spawn points defined in Map")
		panic(err)
	}

	// Define input configurations for each player slot
	// Player 0: WASD keyboard zone
	// Player 1: Arrow keyboard zone
	// Players 2-3: Gamepads (when connected) or additional keyboard zones
	playerInputConfigs := []factory2.PlayerInputConfig{
		{PlayerIndex: 0, GamepadID: nil, KeyboardZone: components.KeyboardZoneWASD},
		{PlayerIndex: 1, GamepadID: nil, KeyboardZone: components.KeyboardZoneArrows},
	}

	numPlayers := len(playerInputConfigs)
	numSpawns := len(levelData.CurrentLevel.PlayerSpawns)

	var firstPlayerSpawn assets.PlayerSpawn
	for i := 0; i < numPlayers; i++ {
		// Use spawn point if available, otherwise offset from first spawn
		var spawnX, spawnY float64
		if i < numSpawns {
			spawn := levelData.CurrentLevel.PlayerSpawns[i]
			spawnX, spawnY = spawn.X, spawn.Y
		} else {
			// Offset from first spawn point (30 pixels apart horizontally)
			spawn := levelData.CurrentLevel.PlayerSpawns[0]
			spawnX = spawn.X + float64(i)*30.0
			spawnY = spawn.Y
		}
		if i == 0 {
			firstPlayerSpawn = levelData.CurrentLevel.PlayerSpawns[0]
		}
		player := factory2.CreatePlayer(ps.ecs, spawnX, spawnY, playerInputConfigs[i])
		playerObj := components.Object.Get(player)
		space.Add(playerObj.Object)
	}

	// Snap camera to first player's start position to prevent panning from (0,0)
	if cameraEntry, ok := components.Camera.First(ps.ecs.World); ok {
		camera := components.Camera.Get(cameraEntry)
		camera.Position.X = firstPlayerSpawn.X
		camera.Position.Y = firstPlayerSpawn.Y
	}

	// Spawn enemies for the current level
	for _, spawn := range levelData.CurrentLevel.EnemySpawns {
		// Use the enemy type from the spawn data, default to "Guard" if not specified
		enemyType := spawn.EnemyType
		if enemyType == "" {
			enemyType = "Guard"
		}
		enemy := factory2.CreateEnemy(ps.ecs, spawn.X, spawn.Y, spawn.PatrolPath, enemyType)
		enemyObj := components.Object.Get(enemy)
		space.Add(enemyObj.Object)
	}

	// Start level music
	systems.PlayLevelMusic(ps.ecs, levelData.CurrentLevel.Name)
}
