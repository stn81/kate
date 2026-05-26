package oltp

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// columnSet caches the column → field-path mapping for a struct T used by
// InsertBuilder. The cache is keyed on reflect.Type so a given T is
// reflected at most once over the process lifetime.
type columnSet struct {
	names      []string
	fieldPaths [][]int
}

var columnSetCache sync.Map // reflect.Type → *columnSet

func columnsForType[T any]() (*columnSet, error) {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		return nil, fmt.Errorf("kate/oltp: T must be a concrete struct type")
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("kate/oltp: T must be a struct, got %s", t.Kind())
	}
	if v, ok := columnSetCache.Load(t); ok {
		return v.(*columnSet), nil
	}
	cs := &columnSet{}
	collectInsertableFields(t, nil, cs)
	columnSetCache.Store(t, cs)
	return cs, nil
}

func collectInsertableFields(t reflect.Type, prefix []int, out *columnSet) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		path := append(append([]int(nil), prefix...), i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			collectInsertableFields(f.Type, path, out)
			continue
		}
		if !f.IsExported() {
			continue
		}
		col, skip, auto := parseTag(f)
		if skip {
			continue
		}
		if auto {
			// auto-increment / serial columns are omitted from the INSERT
			// column list so the DB assigns them.
			continue
		}
		if col == "" {
			col = snakeCase(f.Name)
		}
		out.names = append(out.names, col)
		out.fieldPaths = append(out.fieldPaths, path)
	}
}

// parseTag returns (columnName, skip, isAuto). Mirrors the lookup logic
// used by the db.Scan layer so insert columns line up with scan targets.
func parseTag(f reflect.StructField) (col string, skip bool, auto bool) {
	if tag, ok := f.Tag.Lookup("db"); ok {
		if tag == "-" {
			return "", true, false
		}
		// db:"col,auto" — comma-separated options
		parts := strings.Split(tag, ",")
		col = parts[0]
		for _, p := range parts[1:] {
			if p == "auto" || p == "pk_auto" || p == "autoincr" {
				auto = true
			}
		}
		return col, false, auto
	}
	if tag, ok := f.Tag.Lookup("orm"); ok {
		for _, part := range strings.Split(tag, ";") {
			part = strings.TrimSpace(part)
			if part == "-" {
				return "", true, false
			}
			if part == "auto" {
				auto = true
				continue
			}
			if strings.HasPrefix(part, "column(") && strings.HasSuffix(part, ")") {
				col = part[len("column(") : len(part)-1]
			}
		}
		return col, false, auto
	}
	return "", false, false
}

func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}
