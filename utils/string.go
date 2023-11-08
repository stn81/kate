package utils

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
