package config

// BotDifficulty affects reaction time and decision quality
type BotDifficulty int

const (
	BotDifficultyEasy BotDifficulty = iota
	BotDifficultyNormal
	BotDifficultyHard
)

// BotDifficultyConfig holds tuning values for bot behavior at a specific difficulty
type BotDifficultyConfig struct {
	ReactionDelay    int     // Frames to delay reactions
	AttackRange      float64 // Distance to start attacking
	ChaseRange       float64 // Distance to start chasing
	RetreatThreshold float64 // Health % to start retreating
}

// BotConfigData holds all bot-related configuration
type BotConfigData struct {
	Difficulties map[BotDifficulty]BotDifficultyConfig
}

// Bot holds bot AI configuration
var Bot BotConfigData

func init() {
	Bot = BotConfigData{
		Difficulties: map[BotDifficulty]BotDifficultyConfig{
			BotDifficultyEasy: {
				ReactionDelay:    30,   // 0.5 second reaction time
				AttackRange:      40.0,
				ChaseRange:       150.0,
				RetreatThreshold: 0.2, // Retreat at 20% health
			},
			BotDifficultyNormal: {
				ReactionDelay:    15,   // 0.25 second reaction time
				AttackRange:      50.0,
				ChaseRange:       200.0,
				RetreatThreshold: 0.3, // Retreat at 30% health
			},
			BotDifficultyHard: {
				ReactionDelay:    5,    // Near-instant reaction
				AttackRange:      60.0,
				ChaseRange:       250.0,
				RetreatThreshold: 0.15, // Retreat at 15% health
			},
		},
	}
}
