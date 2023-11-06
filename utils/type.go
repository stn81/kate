package utils

import (
	"reflect"
)

// IsType check type
func IsType(value reflect.Value, expected reflect.Type) bool {
	if !value.IsValid() {
		return false
	}

	typ := value.Type()
	kind := value.Kind()
	if kind == reflect.Pointer {
		typ = value.Type().Elem()
	}
	return typ == expected
}
