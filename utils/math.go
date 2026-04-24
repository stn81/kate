package utils

import "math"

// SafeDiv 安全除法：除以0时返回 defaultValue
func SafeDiv[T float64 | int | int64 | uint64](a, b T, defaultValue T) T {
	if b == 0 {
		return defaultValue
	}
	return a / b
}

func Round(f float64, decimal int) float64 {
	if decimal < 0 {
		return f
	}
	shift := math.Pow10(decimal)
	return math.Round(f*shift) / shift
}
