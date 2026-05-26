// Package chexpr provides ClickHouse-specific typed expressions: bitmap
// operations, array helpers, and the phantom marker types BitMap64 / Array[T]
// that the type system uses to enforce shape correctness on bitmap pipelines.
//
// Bitmap pipelines in report-api look like:
//   userid IN (SELECT arrayJoin(bitmapToArray(bitmapAnd(bm_a, bm_b))))
// which in kate is:
//   sql.InExpr(reg.T.Userid,
//       chexpr.ArrayJoin[uint64](chexpr.BitmapToArray(
//           chexpr.BitmapAnd(bmA, bmB))))
// — every node is typed, so swapping uint64 ↔ string would fail to compile.
package chexpr

import (
	ksql "github.com/stn81/kate/orm/sql"
)

// BitMap64 is the phantom marker for a CK AggregateFunction(groupBitmap, UInt64)
// column or the result of bitmap* functions. Never instantiated directly;
// only used as the type parameter of Expr[BitMap64].
type BitMap64 struct{ _ uint64 }

// Array[T] is the phantom marker for a CK Array(T) column or the result of
// array-returning functions. Used as Expr[Array[T]] in typed pipelines.
type Array[T any] struct{ _ T }

// ----- bitmap operations -----

// BitmapAnd(a, b) — intersection of two bitmaps.
func BitmapAnd(a, b ksql.Expr[BitMap64]) ksql.Expr[BitMap64] {
	return ksql.Func[BitMap64]("bitmapAnd", a, b)
}

// BitmapOr(a, b) — union of two bitmaps.
func BitmapOr(a, b ksql.Expr[BitMap64]) ksql.Expr[BitMap64] {
	return ksql.Func[BitMap64]("bitmapOr", a, b)
}

// BitmapXor(a, b) — symmetric difference.
func BitmapXor(a, b ksql.Expr[BitMap64]) ksql.Expr[BitMap64] {
	return ksql.Func[BitMap64]("bitmapXor", a, b)
}

// BitmapAndnot(a, b) — set subtraction (a \ b).
func BitmapAndnot(a, b ksql.Expr[BitMap64]) ksql.Expr[BitMap64] {
	return ksql.Func[BitMap64]("bitmapAndnot", a, b)
}

// BitmapCardinality(b) — number of set bits as Expr[uint64].
func BitmapCardinality(b ksql.Expr[BitMap64]) ksql.Expr[uint64] {
	return ksql.Func[uint64]("bitmapCardinality", b)
}

// BitmapContains(b, v) — boolean: does b contain v?
func BitmapContains(b ksql.Expr[BitMap64], v ksql.Expr[uint64]) ksql.Expr[bool] {
	return ksql.Func[bool]("bitmapContains", b, v)
}

// BitmapHasAny(a, b) — any element in common?
func BitmapHasAny(a, b ksql.Expr[BitMap64]) ksql.Expr[bool] {
	return ksql.Func[bool]("bitmapHasAny", a, b)
}

// BitmapHasAll(a, b) — does a contain every element of b?
func BitmapHasAll(a, b ksql.Expr[BitMap64]) ksql.Expr[bool] {
	return ksql.Func[bool]("bitmapHasAll", a, b)
}

// BitmapToArray(b) — convert bitmap into Array(UInt64).
func BitmapToArray(b ksql.Expr[BitMap64]) ksql.Expr[Array[uint64]] {
	return ksql.Func[Array[uint64]]("bitmapToArray", b)
}

// BitmapBuild(arr) — build a bitmap from an Array(UInt64).
func BitmapBuild(arr ksql.Expr[Array[uint64]]) ksql.Expr[BitMap64] {
	return ksql.Func[BitMap64]("bitmapBuild", arr)
}

// GroupBitmap is the aggregate that constructs a bitmap from a uint64 column.
func GroupBitmap(uid ksql.Expr[uint64]) ksql.Expr[BitMap64] {
	return ksql.Func[BitMap64]("groupBitmap", uid)
}

// GroupBitmapState is the partial-aggregate variant (returns the state
// suitable for further aggregation via -Merge).
func GroupBitmapState(uid ksql.Expr[uint64]) ksql.Expr[BitMap64] {
	return ksql.Func[BitMap64]("groupBitmapState", uid)
}

// ----- array helpers -----

// ArrayJoin(a) — lateral expansion: emit one row per element.
// The result type is T (the element of Array[T]).
func ArrayJoin[T any](a ksql.Expr[Array[T]]) ksql.Expr[T] {
	return ksql.Func[T]("arrayJoin", a)
}

// Length(a) — array cardinality as Expr[uint64].
func Length[T any](a ksql.Expr[Array[T]]) ksql.Expr[uint64] {
	return ksql.Func[uint64]("length", a)
}

// Has(a, v) — boolean: is v in a?
func Has[T any](a ksql.Expr[Array[T]], v ksql.Expr[T]) ksql.Expr[bool] {
	return ksql.Func[bool]("has", a, v)
}

// EmptyArrayUInt64 produces an empty Array(UInt64) literal, useful as a
// COALESCE fallback when a left-joined array column may be NULL.
func EmptyArrayUInt64() ksql.Expr[Array[uint64]] {
	return ksql.RawExpr[Array[uint64]]("emptyArrayUInt64()")
}

// ----- predicate convenience -----

// ArrayJoinBitmap is the convenience composition for the most common
// "userid IN (...bitmap...)" pattern in report-api:
//   userid IN (SELECT arrayJoin(bitmapToArray(bm)))
//
// Returns an Expr[uint64] that can be used as RHS of an IN predicate.
func ArrayJoinBitmap(bm ksql.Expr[BitMap64]) ksql.Expr[uint64] {
	return ArrayJoin[uint64](BitmapToArray(bm))
}
