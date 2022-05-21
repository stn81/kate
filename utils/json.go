package utils

import (
	"encoding/json"

	"github.com/stn81/dynamic"
)

// ToJSON return json encoded string of interface.
func ToJSON(v interface{}) string {
	var (
		data []byte
		err  error
	)
	if data, err = json.Marshal(v); err != nil {
		return "encoding failure"
	}
	return string(data)
}

// ParseJSON parse json with dynamic field parse support
func ParseJSON(data []byte, ptr interface{}) error {
	return dynamic.ParseJSON(data, ptr)
}
