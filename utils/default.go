package utils

import (
	"fmt"
	"reflect"
)

// SetDefaults set the default value by tag `default:"value"`
func SetDefaults(ptr any) error {
	val := reflect.ValueOf(ptr)
	ind := reflect.Indirect(val)
	typ := ind.Type()
	fullName := typ.PkgPath() + "." + typ.Name()

	if val.Kind() != reflect.Pointer {
		panic(fmt.Errorf("SetDefaults: cannot use non-ptr struct `%s`", fullName))
	}

	if typ.Kind() != reflect.Struct {
		panic(fmt.Errorf("SetDefaults: only allow ptr of struct"))
	}

	numField := ind.NumField()
	for i := 0; i < numField; i++ {
		structField := typ.Field(i)
		defaultValue := structField.Tag.Get("default")
		if defaultValue == "" {
			continue
		}

		field := ind.Field(i)
		if !field.CanSet() {
			continue
		}

		if !field.IsZero() {
			continue
		}

		if err := bindValue(field, defaultValue); err != nil {
			return err
		}
	}
	return nil
}
