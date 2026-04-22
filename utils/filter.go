package utils

// Filter out keys in params
func Filter(params map[string]any, filters []string) {
	for _, f := range filters {
		delete(params, f)
	}
}

// FilterSlice remove specified element
func FilterSlice[S ~[]E, E comparable](slice S, value E) S {
	result := make(S, 0, len(slice))

	for _, item := range slice {
		if item != value {
			result = append(result, item)
		}
	}

	return result
}
