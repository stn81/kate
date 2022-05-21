package utils

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMinMax(t *testing.T) {
	minInt := Min(1, 2, 3, 4, 5)
	require.Equal(t, 1, minInt, "Min(...int)")
	maxInt := Max(1, 2, 3, 4, 5)
	require.Equal(t, 5, maxInt, "Max(...int)")
	minFloat64 := Min([]float64{1.1, 2.1, 3.1, 4.1, 5.1}...)
	require.Equal(t, 1.1, minFloat64, "Min(...float64)")
	maxFloat64 := Max([]float64{1.1, 2.1, 3.1, 4.1, 5.1}...)
	require.Equal(t, 5.1, maxFloat64, "Max(...float64)")
}
