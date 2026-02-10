package gamemath

// ApplyFriction reduces speed toward zero by friction amount.
func ApplyFriction(speedX, friction float64) float64 {
	if speedX > friction {
		return speedX - friction
	}
	if speedX < -friction {
		return speedX + friction
	}
	return 0
}

// ClampSpeed clamps a value to [-max, max].
func ClampSpeed(speed, max float64) float64 {
	if speed > max {
		return max
	}
	if speed < -max {
		return -max
	}
	return speed
}

// CalculateAimDirection returns an aim vector from input state.
// facingX is the player's facing direction (-1 or 1).
// Returns (aimX, aimY) representing the throw direction.
func CalculateAimDirection(facingX float64, upPressed, downPressed, movingHorizontally bool) (aimX, aimY float64) {
	if upPressed && !downPressed {
		if movingHorizontally {
			return facingX, -1.0
		}
		return 0, -1.0
	}
	if downPressed && !upPressed {
		if movingHorizontally {
			return facingX, 1.0
		}
		return 0, 1.0
	}
	return facingX, 0
}
