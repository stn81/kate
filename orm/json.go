package orm

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/stn81/dynamic"
)

// JSONValue is the json wrapper
type JSONValue struct {
	addr      any
	omitEmpty bool
}

func newJSONValue(v any, omitEmpty bool) any {
	return &JSONValue{
		addr:      v,
		omitEmpty: omitEmpty,
	}
}

// Value implements sql.Valuer interface
func (jv *JSONValue) Value() (driver.Value, error) {
	if jv.omitEmpty {
		if jv.addr == nil {
			return "", nil
		}

		if reflect.ValueOf(jv.addr).IsZero() {
			return "", nil
		}
	}

	data, err := json.Marshal(jv.addr)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// Scan implements sql.Scanner interface
func (jv *JSONValue) Scan(value any) error {
	switch rawVal := value.(type) {
	case string:
		if len(rawVal) == 0 {
			return nil
		}
		return dynamic.ParseJSON([]byte(rawVal), jv.addr)
	case []byte:
		if len(rawVal) == 0 {
			return nil
		}
		return dynamic.ParseJSON(rawVal, jv.addr)
	default:
		return errors.New("invalid type for json raw data")
	}
}
