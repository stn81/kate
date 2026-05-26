package sql

// OrderTerm is one entry of an ORDER BY clause: an expression plus
// direction. SelectBuilder.OrderBy takes a variadic []OrderTerm so each
// entry's direction is explicit.
type OrderTerm struct {
	Expr AnyExpr
	Desc bool
}

// Asc returns an ascending ORDER BY entry for the typed expression.
func Asc[T any](e Expr[T]) OrderTerm { return OrderTerm{Expr: e} }

// Desc returns a descending ORDER BY entry.
func Desc[T any](e Expr[T]) OrderTerm { return OrderTerm{Expr: e, Desc: true} }

// AscAny is the existential-form of Asc — for when the expression is
// already type-erased (e.g. from a dim whitelist map).
func AscAny(e AnyExpr) OrderTerm { return OrderTerm{Expr: e} }

// DescAny is the existential-form of Desc.
func DescAny(e AnyExpr) OrderTerm { return OrderTerm{Expr: e, Desc: true} }

// AscAll builds ascending ORDER BY entries for a slice of AnyExpr (mirrors
// the common "user-selected dim list → GROUP BY + ORDER BY" pattern).
func AscAll(es []AnyExpr) []OrderTerm {
	out := make([]OrderTerm, len(es))
	for i, e := range es {
		out[i] = OrderTerm{Expr: e}
	}
	return out
}

func (o OrderTerm) emit(e *Emitter) {
	o.Expr.Emit(e)
	if o.Desc {
		e.WriteString(" DESC")
	}
}
