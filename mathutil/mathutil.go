package mathutil

// ClampFloat constrains a value to the range [min, max].
func ClampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
