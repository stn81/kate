package sql

// Type is a coarse-grained SQL type tag carried by every Expr[T]. It exists
// for build-time shape checks (e.g. Subquery[T] verifying the inner SELECT
// projects a column of T), not for SQL emission — the actual SQL type is
// derived from the operand types or written verbatim by Cast.
type Type int

const (
	TypeUnknown Type = iota
	TypeBool
	TypeInt
	TypeUint
	TypeFloat
	TypeString
	TypeBytes
	TypeTime
	TypeDate
	TypeArray
	TypeMap
	TypeBitmap
	TypeAny
)

func (t Type) String() string {
	switch t {
	case TypeBool:
		return "bool"
	case TypeInt:
		return "int"
	case TypeUint:
		return "uint"
	case TypeFloat:
		return "float"
	case TypeString:
		return "string"
	case TypeBytes:
		return "bytes"
	case TypeTime:
		return "time"
	case TypeDate:
		return "date"
	case TypeArray:
		return "array"
	case TypeMap:
		return "map"
	case TypeBitmap:
		return "bitmap"
	case TypeAny:
		return "any"
	}
	return "unknown"
}

// Num is the constraint for numeric column / expression operations.
type Num interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Stringish is the constraint for string-only operations like Like / ILike.
type Stringish interface{ ~string }

// Literal is the marker type for text substitution in db.Raw templates.
// Bare strings cannot become SQL fragments — callers must explicitly cast
// to sql.Literal so every text-substitution site is grep-able.
type Literal string
