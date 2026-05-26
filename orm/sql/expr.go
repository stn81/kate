package sql

// AnyExpr is the existential erasure of Expr[T] — a non-generic interface
// that any typed expression satisfies. Used for heterogeneous lists
// (SELECT projections, GROUP BY, function arguments) where Go's generics
// don't permit `[]Expr[Mixed types]`.
type AnyExpr interface {
	expr()
	Type() Type
	Emit(e *Emitter)
}

// Expr[T] is a typed value-yielding expression. T is the Go type the
// expression evaluates to when scanned into a row struct.
//
// All Expr[T] producers (Col[T], Lit, Func, RawExpr, Coalesce, etc.) provide
// .As so projection aliases attach via a fluent chain — e.g.
//
//	expr.Coalesce(t.X, sql.Lit("")).As("display_x")
//
// .As is intentionally part of the interface so callers holding an
// Expr[T] (returned from helpers) can alias without type assertions.
type Expr[T any] interface {
	AnyExpr
	exprT(T)
	// As returns a copy of the expression with a projection alias attached.
	// The alias only emits in SELECT contexts; WHERE / JOIN / GROUP BY
	// ignore it. Returns Expr[T] so the chain remains typed.
	As(alias string) Expr[T]
}

// AnyCol is AnyExpr that originates from a real column reference (vs. a
// computed expression). Some clauses (GROUP BY in strict modes, USING) want
// columns specifically.
type AnyCol interface {
	AnyExpr
	column()
}

// ---------- literal ----------

// Lit lifts a Go value v into a typed parameterized literal Expr[T]. The
// emitted SQL is a placeholder (`?` / `$N`) — v travels in args, never in
// the SQL text, so SQL injection is structurally impossible.
func Lit[T any](v T) Expr[T] { return litExpr[T]{v: v} }

type litExpr[T any] struct{ v T }

func (litExpr[T]) expr()       {}
func (litExpr[T]) exprT(T)     {}
func (litExpr[T]) Type() Type  { return goTypeTag[T]() }
func (l litExpr[T]) Emit(e *Emitter) {
	e.Param(l.v)
}
func (l litExpr[T]) As(alias string) Expr[T] {
	return AliasedExpr[T]{Inner: l, Alias: alias}
}

// ---------- raw expr ----------

// RawExpr is an escape hatch returning a typed Expr[T] from a literal SQL
// fragment plus bound args. The fragment is emitted verbatim; args are
// parameterized. Prefer Func / typed helpers over RawExpr whenever a typed
// form exists.
func RawExpr[T any](sql string, args ...any) Expr[T] {
	return rawExprT[T]{sql: sql, args: args}
}

type rawExprT[T any] struct {
	sql  string
	args []any
}

func (rawExprT[T]) expr()       {}
func (rawExprT[T]) exprT(T)     {}
func (rawExprT[T]) Type() Type  { return goTypeTag[T]() }
func (r rawExprT[T]) Emit(e *Emitter) {
	// emit sql verbatim, substituting $? markers — keep it simple: we don't
	// parse placeholders here, the fragment is expected to already use
	// flavor placeholders OR consist of values via subsequent Param() calls.
	// Convention: each `?` in sql consumes one arg in order.
	idx := 0
	for i := 0; i < len(r.sql); i++ {
		if r.sql[i] == '?' && idx < len(r.args) {
			e.Param(r.args[idx])
			idx++
		} else {
			_ = e.WriteByte(r.sql[i])
		}
	}
}
func (r rawExprT[T]) As(alias string) Expr[T] {
	return AliasedExpr[T]{Inner: r, Alias: alias}
}

// ---------- func ----------

// Func builds a typed function call expression. The return type T is
// asserted by the caller (we cannot infer SQL function semantics from the
// Go type system) and is propagated through subsequent typed compositions.
func Func[T any](name string, args ...AnyExpr) Expr[T] {
	return funcExpr[T]{name: name, args: args}
}

type funcExpr[T any] struct {
	name string
	args []AnyExpr
}

func (funcExpr[T]) expr()       {}
func (funcExpr[T]) exprT(T)     {}
func (funcExpr[T]) Type() Type  { return goTypeTag[T]() }
func (f funcExpr[T]) Emit(e *Emitter) {
	e.WriteString(f.name)
	_ = e.WriteByte('(')
	for i, a := range f.args {
		if i > 0 {
			e.WriteString(", ")
		}
		a.Emit(e)
	}
	_ = e.WriteByte(')')
}
func (f funcExpr[T]) As(alias string) Expr[T] {
	return AliasedExpr[T]{Inner: f, Alias: alias}
}

// ---------- closure-based factory (for external packages) ----------

// NewExpr is the factory used by external packages (e.g. kate/v2/sql/expr,
// kate/v2/olap/chexpr) to construct typed expressions without having to
// satisfy the unexported marker methods on Expr[T]. The caller supplies
// the emit callback and a coarse Type tag; the returned value implements
// Expr[T] and is composable with all typed helpers.
func NewExpr[T any](emit func(*Emitter), typeTag Type) Expr[T] {
	return wrappedExpr[T]{emit: emit, typ: typeTag}
}

type wrappedExpr[T any] struct {
	emit func(*Emitter)
	typ  Type
}

func (wrappedExpr[T]) expr()       {}
func (wrappedExpr[T]) exprT(T)     {}
func (w wrappedExpr[T]) Type() Type { return w.typ }
func (w wrappedExpr[T]) Emit(e *Emitter) { w.emit(e) }
func (w wrappedExpr[T]) As(alias string) Expr[T] {
	return AliasedExpr[T]{Inner: w, Alias: alias}
}

// NewAnyExpr is the existential-typed analog of NewExpr for when the
// expression result type is genuinely heterogeneous (rare; prefer NewExpr).
func NewAnyExpr(emit func(*Emitter), typeTag Type) AnyExpr {
	return wrappedAnyExpr{emit: emit, typ: typeTag}
}

type wrappedAnyExpr struct {
	emit func(*Emitter)
	typ  Type
}

func (wrappedAnyExpr) expr()      {}
func (w wrappedAnyExpr) Type() Type { return w.typ }
func (w wrappedAnyExpr) Emit(e *Emitter) { w.emit(e) }

// ---------- aliased ----------

// AliasedExpr is a wrapper that emits "<inner> AS <alias>" in SELECT contexts.
// Most expressions get an alias via Col.As / Func.As-style helpers; for the
// generic case use ExprAs.
type AliasedExpr[T any] struct {
	Inner Expr[T]
	Alias string
}

func ExprAs[T any](e Expr[T], alias string) Expr[T] { return AliasedExpr[T]{Inner: e, Alias: alias} }

func (AliasedExpr[T]) expr()      {}
func (AliasedExpr[T]) exprT(T)    {}
func (a AliasedExpr[T]) Type() Type { return a.Inner.Type() }

// As on AliasedExpr replaces the alias (the last call wins), keeping
// a single AS in the emitted SQL.
func (a AliasedExpr[T]) As(alias string) Expr[T] {
	a.Alias = alias
	return a
}
func (a AliasedExpr[T]) Emit(e *Emitter) {
	a.Inner.Emit(e)
	if a.Alias != "" {
		e.WriteString(" AS ")
		e.Quote(a.Alias)
	}
}

// AliasOf returns the alias attached to an AnyExpr if any, else "".
func AliasOf(e AnyExpr) string {
	if a, ok := e.(interface{ aliasName() string }); ok {
		return a.aliasName()
	}
	return ""
}

func (a AliasedExpr[T]) aliasName() string { return a.Alias }
