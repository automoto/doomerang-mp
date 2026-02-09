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
	matchConfig  *MatchConfig
	once         sync.Once
}

// NewPlatformerScene creates a new platformer scene with default configuration
func NewPlatformerScene(sc SceneChanger) *PlatformerScene {
	return &PlatformerScene{sceneChanger: sc, matchConfig: nil}
}

// NewPlatformerSceneWithConfig creates a new platformer scene with lobby configuration
func NewPlatformerSceneWithConfig(sc SceneChanger, config *MatchConfig) *PlatformerScene {
	return &PlatformerScene{sceneChanger: sc, matchConfig: config}
}

func (ps *PlatformerScene) Update() {
	ps.once.Do(ps.configure)
	ps.ecs.Update()

	// Check for match finished - return to menu
	if systems.IsMatchFinished(ps.ecs) {
		ps.sceneChanger.ChangeScene(NewMenuScene(ps.sceneChanger))
		return
	}

	// Check for game over (all players eliminated)
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

	// Load shaders for player tinting
	if err := assets.LoadShaders(); err != nil {
		panic("failed to load shaders: " + err.Error())
	}

	ecs := ecs.NewECS(donburi.NewWorld())

	// Audio system (runs first, even when paused for menu sounds)
	ecs.AddSystem(systems.UpdateAudio)

	// Systems that always run
	ecs.AddSystem(systems.UpdateInput)
	ecs.AddSystem(systems.UpdateBots) // Must run before UpdateMultiPlayerInput
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

	// Match system runs always (handles countdown, timer, results)
	ecs.AddSystem(systems.UpdateMatch)

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
	ecs.AddRenderer(cfg.Default, systems.DrawMatchHUD)
	ecs.AddRenderer(cfg.Default, systems.DrawMessage)
	ecs.AddRenderer(cfg.Default, systems.DrawDebug)
	ecs.AddRenderer(cfg.Default, systems.DrawPause)
	ecs.AddRenderer(cfg.Default, systems.DrawSettingsMenu)

	ps.ecs = ecs

	// Create the level entity and load level data FIRST.
	levelIndex := 0
	if ps.matchConfig != nil {
		levelIndex = ps.matchConfig.LevelIndex
	}
	level := factory2.CreateLevelAtIndex(ps.ecs, levelIndex)
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

	var firstPlayerSpawn assets.PlayerSpawn
	playerIndex := 0
	numPlayers := 0

	// Track which spawns have been used for team-aware assignment
	usedSpawns := make(map[int]bool)

	// Use lobby config if available, otherwise use defaults
	if ps.matchConfig != nil {
		// Spawn players from lobby configuration
		for i := 0; i < 4; i++ {
			slot := &ps.matchConfig.Slots[i]
			if slot.Type == components.SlotEmpty {
				continue
			}

			// Get spawn point based on team and game mode
			spawn := assignSpawnPoint(
				levelData.CurrentLevel.PlayerSpawns,
				i,
				slot.Team,
				ps.matchConfig.GameMode,
				usedSpawns,
			)

			if playerIndex == 0 {
				firstPlayerSpawn = spawn
			}

			switch slot.Type {
			case components.SlotHuman:
				inputCfg := factory2.PlayerInputConfig{
					PlayerIndex:   i,
					GamepadID:     slot.GamepadID,
					KeyboardZone:  slot.KeyboardZone,
					ControlScheme: slot.ControlScheme,
				}
				player := factory2.CreatePlayer(ps.ecs, spawn.X, spawn.Y, inputCfg)
				// Store original spawn for respawning
				playerData := components.Player.Get(player)
				playerData.OriginalSpawnX = spawn.X
				playerData.OriginalSpawnY = spawn.Y

				playerObj := components.Object.Get(player)
				space.Add(playerObj.Object)
			case components.SlotBot:
				bot := factory2.CreateBotPlayer(ps.ecs, spawn.X, spawn.Y, i, slot.BotDifficulty)
				// Store original spawn for respawning
				botData := components.Player.Get(bot)
				botData.OriginalSpawnX = spawn.X
				botData.OriginalSpawnY = spawn.Y

				botObj := components.Object.Get(bot)
				space.Add(botObj.Object)
			}
			playerIndex++
			numPlayers++
		}
	} else {
		// Default configuration (for backwards compatibility)
		playerInputConfigs := []factory2.PlayerInputConfig{
			{PlayerIndex: 0, GamepadID: nil, KeyboardZone: components.KeyboardZoneArrows, ControlScheme: cfg.ControlSchemeA},
			{PlayerIndex: 1, GamepadID: nil, KeyboardZone: components.KeyboardZoneWASD, ControlScheme: cfg.ControlSchemeB},
		}

		botConfigs := []struct {
			playerIndex int
			difficulty  cfg.BotDifficulty
		}{
			{playerIndex: 2, difficulty: cfg.BotDifficultyNormal},
		}

		numHumans := len(playerInputConfigs)
		numBots := len(botConfigs)
		numPlayers = numHumans + numBots

		// Spawn human players
		for i := 0; i < numHumans; i++ {
			// Use assignSpawnPoint for consistent behavior (FFA mode, no teams)
			spawn := assignSpawnPoint(
				levelData.CurrentLevel.PlayerSpawns,
				i,
				-1, // No team in default mode
				cfg.GameModeFreeForAll,
				usedSpawns,
			)

			if playerIndex == 0 {
				firstPlayerSpawn = spawn
			}
			player := factory2.CreatePlayer(ps.ecs, spawn.X, spawn.Y, playerInputConfigs[i])
			// Store original spawn for respawning
			playerData := components.Player.Get(player)
			playerData.OriginalSpawnX = spawn.X
			playerData.OriginalSpawnY = spawn.Y

			playerObj := components.Object.Get(player)
			space.Add(playerObj.Object)
			playerIndex++
		}

		// Spawn bot players
		for _, botCfg := range botConfigs {
			spawn := assignSpawnPoint(
				levelData.CurrentLevel.PlayerSpawns,
				botCfg.playerIndex,
				-1, // No team in default mode
				cfg.GameModeFreeForAll,
				usedSpawns,
			)

			bot := factory2.CreateBotPlayer(ps.ecs, spawn.X, spawn.Y, botCfg.playerIndex, botCfg.difficulty)
			// Store original spawn for respawning
			botData := components.Player.Get(bot)
			botData.OriginalSpawnX = spawn.X
			botData.OriginalSpawnY = spawn.Y

			botObj := components.Object.Get(bot)
			space.Add(botObj.Object)
			playerIndex++
		}
	}

	// Snap camera to first player's start position to prevent panning from (0,0)
	if cameraEntry, ok := components.Camera.First(ps.ecs.World); ok {
		camera := components.Camera.Get(cameraEntry)
		camera.Position.X = firstPlayerSpawn.X
		camera.Position.Y = firstPlayerSpawn.Y
	}

	// Create match entity and start countdown
	createMatchWithConfig(ps.ecs, numPlayers, ps.matchConfig)

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

// assignSpawnPoint returns the spawn point for a given player slot based on team and game mode.
// For 2v2 mode, Team 0 (slots 0,1) gets left spawns, Team 1 (slots 2,3) gets right spawns.
// For other modes, spawns are assigned by slot index.
func assignSpawnPoint(spawns []assets.PlayerSpawn, slotIndex, team int, gameMode cfg.GameModeID, usedSpawns map[int]bool) assets.PlayerSpawn {
	numSpawns := len(spawns)
	if numSpawns == 0 {
		return assets.PlayerSpawn{X: 100, Y: 100} // Fallback
	}

	var spawnIndex int

	if gameMode == cfg.GameMode2v2 && numSpawns >= 4 {
		// 2v2 mode: Team 0 (slots 0,1) gets left spawns, Team 1 (slots 2,3) gets right
		if team == 0 {
			// Team 1: use spawn 0 or 1
			if !usedSpawns[0] {
				spawnIndex = 0
			} else {
				spawnIndex = 1
			}
		} else {
			// Team 2: use spawn 2 or 3
			if !usedSpawns[2] {
				spawnIndex = 2
			} else {
				spawnIndex = 3
			}
		}
	} else {
		// FFA/1v1/Co-op: assign by slot index
		spawnIndex = slotIndex % numSpawns
	}

	usedSpawns[spawnIndex] = true

	if spawnIndex < numSpawns {
		return spawns[spawnIndex]
	}
	return spawns[0]
}

// createMatchWithConfig creates the match entity and initializes match state with optional config
func createMatchWithConfig(e *ecs.ECS, numPlayers int, matchConfig *MatchConfig) {
	matchEntry := e.World.Entry(e.World.Create(components.Match))

	// Determine game mode and duration
	gameMode := cfg.Match.DefaultGameMode
	duration := cfg.Match.DefaultDuration
	if matchConfig != nil {
		gameMode = matchConfig.GameMode
		duration = matchConfig.MatchMinutes * 60 * 60 // Convert minutes to frames (60 fps)
	}

	// Initialize scores for all players
	scores := make([]components.PlayerScore, numPlayers)
	scoreIdx := 0
	if matchConfig != nil {
		for i := 0; i < 4; i++ {
			slot := &matchConfig.Slots[i]
			if slot.Type == components.SlotEmpty {
				continue
			}
			scores[scoreIdx] = components.PlayerScore{
				PlayerIndex: i,
				Team:        slot.Team,
			}
			scoreIdx++
		}
	} else {
		for i := 0; i < numPlayers; i++ {
			scores[i] = components.PlayerScore{
				PlayerIndex: i,
				Team:        -1, // FFA mode
			}
		}
	}

	components.Match.SetValue(matchEntry, components.MatchData{
		State:          cfg.MatchStateCountdown,
		GameMode:       gameMode,
		Timer:          cfg.Match.CountdownDuration,
		Duration:       duration,
		Scores:         scores,
		WinnerIndex:    -2, // No winner yet
		WinningTeam:    -1,
		CountdownValue: 3,
	})
}
