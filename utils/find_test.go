package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindFirst(t *testing.T) {
	v := []int{1, 2, 3, 4, 6, 7, 9}
	n := len(v)
	offset := FindFirst(n, func(i int) bool {
		return v[i] >= 5
	})
	require.Condition(t, func() bool { return offset >= 0 && offset < n })
	require.Equal(t, 6, v[offset])
}

func TestFindLast(t *testing.T) {
	v := []int{1, 2, 3, 4, 6, 7, 9}
	n := len(v)
	offset := FindLast(n, func(i int) bool {
		return v[i] <= 8
	})
	require.Condition(t, func() bool { return offset >= 0 && offset < n })
	require.Equal(t, 7, v[offset])
}
