package codegen

import "strings"

// mapCKToGo translates a verbatim ClickHouse type expression into a Go
// type string plus an extra-import package path (or "" if no extra
// import is needed).
//
// Recognized type families (case-insensitive on the head identifier):
//   - UInt8/16/32/64, Int8/16/32/64, Float32/64, Bool, String, FixedString(N)
//   - Date / Date32 → date.Date
//   - DateTime / DateTime64(N[, 'TZ']) → datetime.DateTime
//   - Decimal(P, S) → string  (kate-gen avoids pulling in a Decimal lib by default)
//   - UUID → string  (callers can swap to uuid.UUID by hand)
//   - LowCardinality(T) → underlying type T
//   - Nullable(T) → *T (with Date / DateTime preserving their kate types)
//   - Array(T) → []T
//   - Map(K, V) → map[K]V
//   - BitMap64 / AggregateFunction(groupBitmap, UInt64) → chexpr.BitMap64
//
// Unknown types fall back to "any" with no import.
func mapCKToGo(raw string) (goType, goImport string) {
	t := strings.TrimSpace(raw)
	// Strip enclosing modifiers we treat as transparent.
	if inner, ok := unwrap(t, "LowCardinality"); ok {
		return mapCKToGo(inner)
	}
	if inner, ok := unwrap(t, "Nullable"); ok {
		gt, gi := mapCKToGo(inner)
		// already-pointer Go types (slices, maps, BitMap64 placeholder)
		// stay unchanged; otherwise prepend *.
		if strings.HasPrefix(gt, "[]") || strings.HasPrefix(gt, "map[") || strings.HasPrefix(gt, "*") {
			return gt, gi
		}
		return "*" + gt, gi
	}
	if inner, ok := unwrap(t, "Array"); ok {
		gt, gi := mapCKToGo(inner)
		return "[]" + gt, gi
	}
	if inner, ok := unwrap(t, "Map"); ok {
		// Map(K, V): split on the top-level comma.
		parts := splitTopLevelCommas(inner)
		if len(parts) != 2 {
			return "any", ""
		}
		kt, ki := mapCKToGo(strings.TrimSpace(parts[0]))
		vt, vi := mapCKToGo(strings.TrimSpace(parts[1]))
		_ = ki
		_ = vi
		// We don't try to merge two extra imports cleanly; emitter handles
		// a single import-per-column. For Map values needing time.Time
		// callers will get the import via the column's own Go type.
		return "map[" + kt + "]" + vt, ""
	}
	if _, ok := unwrap(t, "AggregateFunction"); ok {
		// CK's groupBitmap aggregate stores a uint64 bitmap blob.
		return "chexpr.BitMap64", "github.com/stn81/kate/orm/olap/chexpr"
	}
	// FixedString(N) / DateTime64(N[, 'TZ']) / Decimal(P,S) carry params;
	// strip the param list for the head check.
	head := t
	if i := strings.IndexByte(t, '('); i >= 0 {
		head = t[:i]
	}
	switch head {
	case "Bool":
		return "bool", ""
	case "UInt8":
		return "uint8", ""
	case "UInt16":
		return "uint16", ""
	case "UInt32":
		return "uint32", ""
	case "UInt64":
		return "uint64", ""
	case "Int8":
		return "int8", ""
	case "Int16":
		return "int16", ""
	case "Int32":
		return "int32", ""
	case "Int64":
		return "int64", ""
	case "Float32":
		return "float32", ""
	case "Float64":
		return "float64", ""
	case "String", "FixedString":
		return "string", ""
	case "UUID":
		return "string", ""
	case "Decimal":
		// Default to string for Decimal; callers wanting shopspring/decimal
		// swap by hand after generation.
		return "string", ""
	case "Date", "Date32":
		return "date.Date", "github.com/stn81/kate/datetime/date"
	case "DateTime", "DateTime64":
		return "datetime.DateTime", "github.com/stn81/kate/datetime"
	case "BitMap64":
		return "chexpr.BitMap64", "github.com/stn81/kate/orm/olap/chexpr"
	}
	return "any", ""
}

// unwrap returns (inner, true) if t looks like "Name(inner)", else (_, false).
func unwrap(t, name string) (string, bool) {
	if !strings.HasPrefix(t, name) {
		return "", false
	}
	rest := t[len(name):]
	if len(rest) == 0 || rest[0] != '(' {
		return "", false
	}
	if rest[len(rest)-1] != ')' {
		return "", false
	}
	return rest[1 : len(rest)-1], true
}
