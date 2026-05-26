// Package expr provides typed expression helpers built on top of sql.Expr[T]:
// COALESCE, CASE/WHEN, CAST, aggregate functions, etc.
//
// All helpers are pure constructors — they return Expr[T] AST nodes that
// the SelectBuilder emits at build time. None of them touch a database.
package expr

import (
	ksql "github.com/stn81/kate/orm/sql"
)

// Coalesce returns the first non-null value among its operands. The return
// type is the operand type, propagated through generic constraint matching.
func Coalesce[T any](first ksql.Expr[T], rest ...ksql.Expr[T]) ksql.Expr[T] {
	args := make([]ksql.AnyExpr, 0, 1+len(rest))
	args = append(args, first)
	for _, r := range rest {
		args = append(args, r)
	}
	return ksql.Func[T]("COALESCE", args...)
}

// NullIf returns NULL if a == b, else a.
func NullIf[T any](a, b ksql.Expr[T]) ksql.Expr[T] {
	return ksql.Func[T]("NULLIF", a, b)
}

// ----- CASE / WHEN -----

// CaseWhen is one branch of a CASE expression.
type CaseWhen[T any] struct {
	Cond ksql.Predicate
	Then ksql.Expr[T]
}

// When constructs a typed CASE branch.
func When[T any](cond ksql.Predicate, then ksql.Expr[T]) CaseWhen[T] {
	return CaseWhen[T]{Cond: cond, Then: then}
}

// Case returns a CASE WHEN ... ELSE expression. `els` is the ELSE branch;
// pass sql.Lit(zero) if no semantic ELSE is needed (CK rejects CASE
// without ELSE in many contexts).
func Case[T any](els ksql.Expr[T], whens ...CaseWhen[T]) ksql.Expr[T] {
	whensCopy := append([]CaseWhen[T]{}, whens...)
	elsCopy := els
	emit := func(e *ksql.Emitter) {
		e.WriteString("CASE")
		for _, w := range whensCopy {
			e.WriteString(" WHEN ")
			w.Cond.Emit(e)
			e.WriteString(" THEN ")
			w.Then.Emit(e)
		}
		if elsCopy != nil {
			e.WriteString(" ELSE ")
			elsCopy.Emit(e)
		}
		e.WriteString(" END")
	}
	return ksql.NewExpr[T](emit, ksql.TypeAny)
}

// ----- CAST -----

// Cast forces a SQL-level type conversion: CAST(expr AS sqlType). The Go
// return type U is asserted by the caller; the SQL type string is emitted
// verbatim.
func Cast[U any](e ksql.AnyExpr, sqlType string) ksql.Expr[U] {
	inner := e
	sqlT := sqlType
	emit := func(em *ksql.Emitter) {
		em.WriteString("CAST(")
		inner.Emit(em)
		em.WriteString(" AS ")
		em.WriteString(sqlT)
		_ = em.WriteByte(')')
	}
	return ksql.NewExpr[U](emit, ksql.TypeAny)
}

// ----- aggregates -----

// Sum is the typed SUM aggregate. Return type matches the operand type.
func Sum[T ksql.Num](e ksql.Expr[T]) ksql.Expr[T] {
	return ksql.Func[T]("sum", e)
}

// Count returns the COUNT(expr) aggregate as Expr[uint64].
func Count(e ksql.AnyExpr) ksql.Expr[uint64] {
	return ksql.Func[uint64]("count", e)
}

// CountStar returns COUNT(*) as Expr[uint64].
func CountStar() ksql.Expr[uint64] {
	return ksql.RawExpr[uint64]("count(*)")
}

// CountIf returns countIf(cond) — ClickHouse-friendly conditional count.
// On MySQL/PG `Sum(Case(...))` is more portable; CountIf names the
// CK-native form for callers that know they target CK.
func CountIf(cond ksql.Predicate) ksql.Expr[uint64] {
	return ksql.Func[uint64]("countIf", asExprPred(cond))
}

// Min returns the MIN aggregate.
func Min[T any](e ksql.Expr[T]) ksql.Expr[T] {
	return ksql.Func[T]("min", e)
}

// Max returns the MAX aggregate.
func Max[T any](e ksql.Expr[T]) ksql.Expr[T] {
	return ksql.Func[T]("max", e)
}

// Avg returns AVG as float64.
func Avg[T ksql.Num](e ksql.Expr[T]) ksql.Expr[float64] {
	return ksql.Func[float64]("avg", e)
}

// UniqExact returns uniqExact(e) (ClickHouse exact-distinct aggregate).
func UniqExact(e ksql.AnyExpr) ksql.Expr[uint64] {
	return ksql.Func[uint64]("uniqExact", e)
}

// ----- string helpers -----

// Lower(e): LOWER(e). Restricted to string-kinded T by the constraint.
func Lower[T ksql.Stringish](e ksql.Expr[T]) ksql.Expr[T] {
	return ksql.Func[T]("lower", e)
}

// Upper(e): UPPER(e).
func Upper[T ksql.Stringish](e ksql.Expr[T]) ksql.Expr[T] {
	return ksql.Func[T]("upper", e)
}

// Concat returns CONCAT(args...). Args may be heterogeneous; the result is
// typed as string.
func Concat(args ...ksql.AnyExpr) ksql.Expr[string] {
	return ksql.Func[string]("concat", args...)
}

// ----- helpers -----

// asExprPred wraps a Predicate as AnyExpr by delegating Emit to it.
// Used by countIf-style functions whose argument is a boolean predicate.
func asExprPred(p ksql.Predicate) ksql.AnyExpr {
	pred := p
	return ksql.NewAnyExpr(func(e *ksql.Emitter) {
		pred.Emit(e)
	}, ksql.TypeBool)
}
