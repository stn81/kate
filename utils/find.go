package utils

import "sort"

// FindFirst return the first index that pred(i) is true.
// If not found, return n.
func FindFirst(n int, pred func(i int) bool) int {
	return sort.Search(n, pred)
}

// FindLast return the last index that pred(i) is true.
// If not found, return -1.
func FindLast(n int, pred func(i int) bool) int {
	reverseOffset := sort.Search(n, func(i int) bool {
		j := n - i - 1
		return pred(j)
	})
	return n - reverseOffset - 1
}
