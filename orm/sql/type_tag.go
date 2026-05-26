package sql

import (
	"reflect"
	"sync"
	"time"
)

// goTypeTag returns a coarse Type tag for a Go type T. Result is cached per
// reflect.Type so the reflective walk only happens once per type instance.
func goTypeTag[T any]() Type {
	var zero T
	rt := reflect.TypeOf(zero)
	if rt == nil {
		return TypeAny
	}
	if v, ok := typeTagCache.Load(rt); ok {
		return v.(Type)
	}
	t := classifyType(rt)
	typeTagCache.Store(rt, t)
	return t
}

var typeTagCache sync.Map // reflect.Type → Type

var timeType = reflect.TypeOf(time.Time{})

func classifyType(rt reflect.Type) Type {
	if rt == timeType {
		return TypeTime
	}
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
		if rt == timeType {
			return TypeTime
		}
	}
	switch rt.Kind() {
	case reflect.Bool:
		return TypeBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return TypeInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TypeUint
	case reflect.Float32, reflect.Float64:
		return TypeFloat
	case reflect.String:
		return TypeString
	case reflect.Slice:
		if rt.Elem().Kind() == reflect.Uint8 {
			return TypeBytes
		}
		return TypeArray
	case reflect.Array:
		return TypeArray
	case reflect.Map:
		return TypeMap
	case reflect.Struct:
		// Tagged phantom types (e.g. chexpr.BitMap64) are user-defined structs;
		// their Type is decided by their owning package via custom Expr impls,
		// not by reflection. For an opaque struct we return TypeAny.
		return TypeAny
	}
	return TypeAny
}
