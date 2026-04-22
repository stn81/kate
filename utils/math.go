package utils

// SafeDiv 安全除法：除以0时返回 defaultValue
func SafeDiv[T float64 | int | int64 | uint64](a, b T, defaultValue T) T {
	if b == 0 {
		return defaultValue
	}
	return a / b
}
