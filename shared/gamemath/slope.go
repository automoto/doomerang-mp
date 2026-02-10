package gamemath

import (
	"github.com/automoto/doomerang-mp/mathutil"
	"github.com/solarlune/resolv"
)

// GetSlopeSurfaceY calculates the slope surface Y at the object's center X.
// upRightTag and upLeftTag are the resolv tags used to identify slope direction.
func GetSlopeSurfaceY(object *resolv.Object, ramp *resolv.Object, upRightTag, upLeftTag string) float64 {
	playerCenterX := object.X + object.W/2
	relativeX := mathutil.ClampFloat(playerCenterX-ramp.X, 0, ramp.W)
	slope := relativeX / ramp.W

	if ramp.HasTags(upRightTag) {
		return ramp.Y + ramp.H*(1-slope)
	}
	if ramp.HasTags(upLeftTag) {
		return ramp.Y + ramp.H*slope
	}
	return ramp.Y
}

// SnapToSlopeY returns the Y position to snap an object onto a slope surface.
func SnapToSlopeY(objectH, surfaceY, offset float64) float64 {
	return surfaceY - objectH + offset
}
