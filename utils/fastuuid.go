package utils

import (
	"encoding/hex"

	"github.com/rogpeppe/fastuuid"
)

var uuidGenerator = fastuuid.MustNewGenerator()

// FastUuid generate a new UUID as []byte
func FastUuid() [24]byte {
	return uuidGenerator.Next()
}

// FastUuidStr generate a new UUID as string
func FastUuidStr() string {
	b := uuidGenerator.Next()
	return hex.EncodeToString(b[:])
}
