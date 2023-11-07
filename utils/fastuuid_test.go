package utils

import "testing"

func BenchmarkFastUuid(t *testing.B) {
	for i := 0; i < t.N; i++ {
		FastUuid()
	}
}

func BenchmarkFastUuidStr(t *testing.B) {
	for i := 0; i < t.N; i++ {
		FastUuidStr()
	}
}
