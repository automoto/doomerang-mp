package gamemath

import "math"

// CalculateHomingVelocity returns velocity components to home toward a target.
func CalculateHomingVelocity(boomX, boomY, targetX, targetY, returnSpeed float64) (velX, velY float64) {
	dirX := targetX - boomX
	dirY := targetY - boomY
	dist := math.Sqrt(dirX*dirX + dirY*dirY)
	if dist > 0 {
		velX = (dirX / dist) * returnSpeed
		velY = (dirY / dist) * returnSpeed
	}
	return velX, velY
}

// CalculateThrowVelocity returns initial throw velocity with arc lift.
func CalculateThrowVelocity(aimX, aimY, speed, throwLift float64) (velX, velY float64) {
	return aimX * speed, aimY*speed - throwLift
}

// CalculateMaxRange returns range scaled by charge ratio.
func CalculateMaxRange(baseRange, maxChargeRange, chargeRatio float64) float64 {
	return baseRange + chargeRatio*(maxChargeRange-baseRange)
}

// CalculateDamage returns damage scaled by charge ratio.
func CalculateDamage(baseDamage, maxBonus int, chargeRatio float64) int {
	return baseDamage + int(float64(maxBonus)*chargeRatio)
}

// CalculateThrowSpeed returns speed scaled by charge ratio.
func CalculateThrowSpeed(baseSpeed, chargeRatio float64) float64 {
	return baseSpeed * (1.0 + chargeRatio*0.5)
}
