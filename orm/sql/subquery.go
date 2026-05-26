package sql

// Subquery wraps a SELECT projecting a single column of type T as a typed
// scalar expression. The single-column constraint is enforced at Build
// time (ErrSubqueryShape if the projection count != 1).
func Subquery[T any](sb *SelectBuilder) Expr[T] { return subqueryExpr[T]{sb: sb} }

type subqueryExpr[T any] struct{ sb *SelectBuilder }

func (subqueryExpr[T]) expr()       {}
func (subqueryExpr[T]) exprT(T)     {}
func (subqueryExpr[T]) Type() Type  { return goTypeTag[T]() }
func (s subqueryExpr[T]) Emit(e *Emitter) {
	if n := s.sb.SelectColCount(); n != 1 && n != -1 {
		e.SetError(ErrSubqueryShape{Got: n})
		return
	}
	e.WriteByte('(')
	s.sb.emit(e)
	e.WriteByte(')')
}

// InSubquery builds `col IN (SELECT ...)` for a typed column. The subquery
// must project a single column of type T.
func InSubquery[T any](c Col[T], sb *SelectBuilder) Predicate {
	return inSubP{left: stripAlias[T](c), sub: sb}
}

// NotInSubquery builds `col NOT IN (SELECT ...)`.
func NotInSubquery[T any](c Col[T], sb *SelectBuilder) Predicate {
	return inSubP{left: stripAlias[T](c), sub: sb, not: true}
}

// InSubqueryExpr is the expr-on-LHS form of InSubquery, useful when the
// LHS is an arithmetic / function expression rather than a bare column.
func InSubqueryExpr[T any](e Expr[T], sb *SelectBuilder) Predicate {
	return inSubP{left: e, sub: sb}
}
