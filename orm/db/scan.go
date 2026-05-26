package db

import (
	stdsql "database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// rowMapper turns SQL result columns into struct fields. Built once per
// (Go type, column name set) and cached.
type rowMapper struct {
	// fields[i] is the field index path for column i, or nil if the
	// column has no matching struct field (Scan into a *scrap interface).
	fields [][]int
	// scrapCount is the number of columns without a target field; used
	// to size the scrap slice.
	scrapCount int
}

var rowMapperCache sync.Map // mapperKey → *rowMapper

type mapperKey struct {
	t       reflect.Type
	colKey  string // joined "|" of column names
}

// buildMapper inspects type t (must be a struct) and pairs each SQL column
// name to a struct field path. Supported tags: `db:"col"`, `orm:"column(col)"`.
// Fields without a matching column are simply ignored at Scan time.
func buildMapper(t reflect.Type, columns []string) (*rowMapper, error) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("kate/db: target type %s is not a struct", t)
	}
	key := mapperKey{t: t, colKey: strings.Join(columns, "|")}
	if v, ok := rowMapperCache.Load(key); ok {
		return v.(*rowMapper), nil
	}

	// First pass: collect all named fields (recursing into anonymous
	// embeddeds), keyed by column name.
	fieldByCol := map[string][]int{}
	collectFields(t, nil, fieldByCol)

	m := &rowMapper{fields: make([][]int, len(columns))}
	for i, col := range columns {
		if path, ok := fieldByCol[col]; ok {
			m.fields[i] = path
		} else {
			m.scrapCount++
		}
	}
	rowMapperCache.Store(key, m)
	return m, nil
}

// collectFields walks t recursively and records each field's column name
// → index path.
func collectFields(t reflect.Type, prefix []int, out map[string][]int) {
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
			// embedded struct — recurse but don't claim the embedded type
			// name as a column.
			collectFields(f.Type, path, out)
			continue
		}
		if !f.IsExported() {
			continue
		}
		col := columnNameFromTag(f)
		if col == "" || col == "-" {
			continue
		}
		out[col] = path
	}
}

// columnNameFromTag resolves the SQL column name for a struct field.
// Priority: `db:"col"` > `orm:"column(col)"` > kebab-cased field name.
func columnNameFromTag(f reflect.StructField) string {
	if tag, ok := f.Tag.Lookup("db"); ok {
		if i := strings.IndexByte(tag, ','); i >= 0 {
			tag = tag[:i]
		}
		return tag
	}
	if tag, ok := f.Tag.Lookup("orm"); ok {
		// orm:"column(name);pk;auto" — extract column(...)
		for _, part := range strings.Split(tag, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "column(") && strings.HasSuffix(part, ")") {
				return part[len("column(") : len(part)-1]
			}
			if part == "-" {
				return "-"
			}
		}
	}
	// fall back to snake_case of the field name
	return camelToSnake(f.Name)
}

func camelToSnake(s string) string {
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

// scanInto walks rows and populates dst (must be *[]T or *[]*T). For each
// row a new T is allocated, fields populated by mapper, and appended.
// stopAfter limits row count; 0 = unlimited. firstOnly causes scanInto to
// stop after exactly one row.
//
// Returns the number of rows scanned and any error encountered.
func scanInto(rows *stdsql.Rows, m *rowMapper, columns []string, dstSlice reflect.Value, elemType reflect.Type, ptrElem bool, firstOnly bool) (int, error) {
	count := 0
	scanBuf := make([]any, len(columns))
	for rows.Next() {
		newElem := reflect.New(elemType).Elem()
		for i := 0; i < len(columns); i++ {
			path := m.fields[i]
			if path == nil {
				var scrap any
				scanBuf[i] = &scrap
				continue
			}
			fv := newElem
			for _, idx := range path {
				if fv.Kind() == reflect.Pointer {
					if fv.IsNil() {
						fv.Set(reflect.New(fv.Type().Elem()))
					}
					fv = fv.Elem()
				}
				fv = fv.Field(idx)
			}
			scanBuf[i] = fv.Addr().Interface()
		}
		if err := rows.Scan(scanBuf...); err != nil {
			return count, err
		}
		if ptrElem {
			dstSlice.Set(reflect.Append(dstSlice, newElem.Addr()))
		} else {
			dstSlice.Set(reflect.Append(dstSlice, newElem))
		}
		count++
		if firstOnly {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return count, err
	}
	return count, nil
}
