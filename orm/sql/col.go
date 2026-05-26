package sql

// Col[T] is a typed column reference. The generic parameter T is the Go
// type the column scans into; it carries through to predicate operands
// (Eq, In, Between, ...) and to typed function compositions so that
// `userid.Eq("abc")` fails at compile time.
type Col[T any] struct {
	schema string // "" if unqualified
	table  string // table name (for un-aliased references)
	alias  string // alias name (overrides table prefix when non-empty)
	name   string // column name
	// asAlias renames the column in a SELECT projection: SELECT col AS asAlias.
	asAlias string
}

// NewCol constructs a typed column reference. Typically emitted by codegen.
func NewCol[T any](schema, table, name string) Col[T] {
	return Col[T]{schema: schema, table: table, name: name}
}

// As returns a copy of the column with a projection alias attached. The
// alias is only used when the column appears in a SELECT list.
func (c Col[T]) As(alias string) Col[T] {
	c.asAlias = alias
	return c
}

// Of requalifies the column under the given TableRef's alias (or schema/table
// if no alias is set). Use when the same column descriptor needs to refer to
// the same table under different aliases in a self-join.
func (c Col[T]) Of(ref TableRef) Col[T] {
	c.schema = ref.schema
	c.table = ref.name
	c.alias = ref.alias
	return c
}

// Name reports the bare column name.
func (c Col[T]) Name() string { return c.name }

// ----- AnyExpr / AnyCol / Expr[T] interfaces -----

func (Col[T]) expr()       {}
func (Col[T]) exprT(T)     {}
func (Col[T]) column()     {}
func (Col[T]) Type() Type  { return goTypeTag[T]() }
func (c Col[T]) Emit(e *Emitter) {
	e.QualifiedColumn(c.schema, c.table, c.alias, c.name)
	if c.asAlias != "" {
		e.WriteString(" AS ")
		e.Quote(c.asAlias)
	}
}

// ----- comparison predicates: typed -----

// Eq builds `c = v` with v parameterized.
func (c Col[T]) Eq(v T) Predicate {
	return binOpP{left: stripAlias[T](c), op: "=", right: Lit[T](v)}
}

// Neq builds `c <> v`.
func (c Col[T]) Neq(v T) Predicate {
	return binOpP{left: stripAlias[T](c), op: "<>", right: Lit[T](v)}
}

// Lt builds `c < v`.
func (c Col[T]) Lt(v T) Predicate {
	return binOpP{left: stripAlias[T](c), op: "<", right: Lit[T](v)}
}

// Lte builds `c <= v`.
func (c Col[T]) Lte(v T) Predicate {
	return binOpP{left: stripAlias[T](c), op: "<=", right: Lit[T](v)}
}

// Gt builds `c > v`.
func (c Col[T]) Gt(v T) Predicate {
	return binOpP{left: stripAlias[T](c), op: ">", right: Lit[T](v)}
}

// Gte builds `c >= v`.
func (c Col[T]) Gte(v T) Predicate {
	return binOpP{left: stripAlias[T](c), op: ">=", right: Lit[T](v)}
}

// EqExpr builds `c = expr` with another typed expression on the RHS.
func (c Col[T]) EqExpr(other Expr[T]) Predicate {
	return binOpP{left: stripAlias[T](c), op: "=", right: other}
}

// NeqExpr builds `c <> expr`.
func (c Col[T]) NeqExpr(other Expr[T]) Predicate {
	return binOpP{left: stripAlias[T](c), op: "<>", right: other}
}

// LtExpr builds `c < expr`.
func (c Col[T]) LtExpr(other Expr[T]) Predicate {
	return binOpP{left: stripAlias[T](c), op: "<", right: other}
}

// GtExpr builds `c > expr`.
func (c Col[T]) GtExpr(other Expr[T]) Predicate {
	return binOpP{left: stripAlias[T](c), op: ">", right: other}
}

// In builds `c IN (v1, v2, ...)`. Variadic form is convenient for literal
// lists; for an already-built slice prefer InSlice to avoid the spread cost.
func (c Col[T]) In(vs ...T) Predicate {
	return c.InSlice(vs)
}

// InSlice is the slice form of In.
func (c Col[T]) InSlice(vs []T) Predicate {
	if len(vs) == 0 {
		return RawPredicate("FALSE")
	}
	args := make([]any, len(vs))
	for i, v := range vs {
		args[i] = v
	}
	return inP{left: stripAlias[T](c), values: args}
}

// NotIn builds `c NOT IN (v1, v2, ...)`.
func (c Col[T]) NotIn(vs ...T) Predicate {
	if len(vs) == 0 {
		return RawPredicate("TRUE")
	}
	args := make([]any, len(vs))
	for i, v := range vs {
		args[i] = v
	}
	return inP{left: stripAlias[T](c), values: args, not: true}
}

// Between builds `c BETWEEN lo AND hi` (both parameterized).
func (c Col[T]) Between(lo, hi T) Predicate {
	return betweenP{left: stripAlias[T](c), lo: lo, hi: hi}
}

// IsNull builds `c IS NULL`.
func (c Col[T]) IsNull() Predicate { return nullP{left: stripAlias[T](c)} }

// IsNotNull builds `c IS NOT NULL`.
func (c Col[T]) IsNotNull() Predicate { return nullP{left: stripAlias[T](c), not: true} }

// ----- string-only operations via generic constraint -----

// Like emits `col LIKE pat`. Constrained to string-kinded T at compile time:
// `Like(userid_col, "...")` fails to compile when userid_col is Col[uint64].
func Like[T Stringish](c Col[T], pat string) Predicate {
	return binOpP{left: stripAlias[T](c), op: "LIKE", right: Lit(pat)}
}

// NotLike emits `col NOT LIKE pat`.
func NotLike[T Stringish](c Col[T], pat string) Predicate {
	return binOpP{left: stripAlias[T](c), op: "NOT LIKE", right: Lit(pat)}
}

// stripAlias returns a copy of the column without its projection alias so
// that the predicate's LHS emits the bare reference (we don't want
// `WHERE col AS x = ?`).
func stripAlias[T any](c Col[T]) Col[T] {
	c.asAlias = ""
	return c
}

// ----- numeric ops returning Expr[T] -----

// Add returns `a + b` typed as T.
func Add[T Num](a, b Expr[T]) Expr[T] {
	return binExpr[T]{left: a, op: "+", right: b}
}

// Sub returns `a - b`.
func Sub[T Num](a, b Expr[T]) Expr[T] {
	return binExpr[T]{left: a, op: "-", right: b}
}

// Mul returns `a * b`.
func Mul[T Num](a, b Expr[T]) Expr[T] {
	return binExpr[T]{left: a, op: "*", right: b}
}

// Div returns `a / b`.
func Div[T Num](a, b Expr[T]) Expr[T] {
	return binExpr[T]{left: a, op: "/", right: b}
}

type binExpr[T any] struct {
	left  Expr[T]
	op    string
	right Expr[T]
}

func (binExpr[T]) expr()       {}
func (binExpr[T]) exprT(T)     {}
func (binExpr[T]) Type() Type  { return goTypeTag[T]() }
func (b binExpr[T]) Emit(e *Emitter) {
	e.WriteByte('(')
	b.left.Emit(e)
	e.WriteByte(' ')
	e.WriteString(b.op)
	e.WriteByte(' ')
	b.right.Emit(e)
	e.WriteByte(')')
}
