package utils

import (
	"sort"
	"strings"
)

// Much efficient than regexp, about 20x

// IsAllNumbers return true if input contains only numbers.
func IsAllNumbers(input string) bool {
	for _, v := range input {
		if v < '0' || v > '9' {
			return false
		}
	}
	return true
}

// IsAllLetters return true if input contains only letters.
func IsAllLetters(input string) bool {
	for _, v := range input {
		if !((v >= 'A' && v <= 'Z') || (v >= 'a' && v <= 'z')) {
			return false
		}
	}
	return true
}

// IsAllNumberLetters return true if input contains only numbers or letters.
func IsAllNumberLetters(input string) bool {
	for _, v := range input {
		if !((v >= 'A' && v <= 'Z') || (v >= 'a' && v <= 'z') || (v >= '0' && v <= '9')) {
			return false
		}
	}
	return true
}

// FindAllPrefixMatch return all item in sorted string slice has the specified prefix.
func FindAllPrefixMatch(ss []string, prefix string) []string {
	n := len(ss)
	prefixLen := len(prefix)
	offset, found := sort.Find(n, func(i int) int {
		if len(ss[i]) >= prefixLen {
			return strings.Compare(prefix, ss[i][:prefixLen])
		} else {
			return 1
		}
	})
	if !found {
		return nil
	}

	var result []string
	for i := offset; i < n; i++ {
		if !strings.HasPrefix(ss[i], prefix) {
			break
		}
		result = append(result, ss[i])
	}
	return result
}
