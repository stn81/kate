package utils

// Filter out keys in params
func Filter(params map[string]interface{}, filters []string) {
	for _, f := range filters {
		delete(params, f)
	}
}
