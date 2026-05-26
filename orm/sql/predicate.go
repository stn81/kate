package sql

// Predicate is the type of boolean SQL fragments used in WHERE / HAVING /
// JOIN ON / Prewhere / CASE WHEN. Predicates are AND-combined by default
// at the clause level; use And/Or/Not for explicit composition.
type Predicate interface {
	predicate()
	Emit(e *Emitter)
}

// ---------- composition ----------

// And combines predicates with AND. Empty input emits TRUE.
func And(ps ...Predicate) Predicate { return andP(ps) }

// Or combines predicates with OR. Empty input emits FALSE.
func Or(ps ...Predicate) Predicate { return orP(ps) }

// Not negates a predicate.
func Not(p Predicate) Predicate { return notP{p} }

type andP []Predicate

func (andP) predicate() {}
func (ps andP) Emit(e *Emitter) {
	if len(ps) == 0 {
		e.WriteString("TRUE")
		return
	}
	if len(ps) == 1 {
		ps[0].Emit(e)
		return
	}
	for i, p := range ps {
		if i > 0 {
			e.WriteString(" AND ")
		}
		e.WriteByte('(')
		p.Emit(e)
		e.WriteByte(')')
	}
}

type orP []Predicate

func (orP) predicate() {}
func (ps orP) Emit(e *Emitter) {
	if len(ps) == 0 {
		e.WriteString("FALSE")
		return
	}
	if len(ps) == 1 {
		ps[0].Emit(e)
		return
	}
	for i, p := range ps {
		if i > 0 {
			e.WriteString(" OR ")
		}
		e.WriteByte('(')
		p.Emit(e)
		e.WriteByte(')')
	}
}

type notP struct{ p Predicate }

func (notP) predicate() {}
func (n notP) Emit(e *Emitter) {
	e.WriteString("NOT (")
	n.p.Emit(e)
	e.WriteByte(')')
}

// ---------- raw predicate ----------

// RawPredicate is the escape hatch for boolean fragments unrepresentable
// by typed predicates. Args are parameterized; `?` in sql consumes args
// in order.
func RawPredicate(sql string, args ...any) Predicate {
	return rawPred{sql: sql, args: args}
}

type rawPred struct {
	sql  string
	args []any
}

func (rawPred) predicate() {}
func (r rawPred) Emit(e *Emitter) {
	idx := 0
	for i := 0; i < len(r.sql); i++ {
		if r.sql[i] == '?' && idx < len(r.args) {
			e.Param(r.args[idx])
			idx++
		} else {
			e.WriteByte(r.sql[i])
		}
	}
}

// ---------- binary predicate (col OP value/expr) ----------

type binOpP struct {
	left  AnyExpr
	op    string // "=" | "<>" | "<" | "<=" | ">" | ">=" | "LIKE" | "ILIKE"
	right AnyExpr
}

func (binOpP) predicate() {}
func (b binOpP) Emit(e *Emitter) {
	b.left.Emit(e)
	e.WriteByte(' ')
	e.WriteString(b.op)
	e.WriteByte(' ')
	b.right.Emit(e)
}

// ---------- IN predicate ----------

type inP struct {
	left   AnyExpr
	values []any // values, will become parameterized placeholders
	not    bool
}

func (inP) predicate() {}
func (in inP) Emit(e *Emitter) {
	in.left.Emit(e)
	if in.not {
		e.WriteString(" NOT IN (")
	} else {
		e.WriteString(" IN (")
	}
	for i, v := range in.values {
		if i > 0 {
			e.WriteString(", ")
		}
		e.Param(v)
	}
	e.WriteByte(')')
}

// ---------- IN (subquery) ----------

type inSubP struct {
	left AnyExpr
	sub  *SelectBuilder
	not  bool
}

func (inSubP) predicate() {}
func (in inSubP) Emit(e *Emitter) {
	in.left.Emit(e)
	if in.not {
		e.WriteString(" NOT IN (")
	} else {
		e.WriteString(" IN (")
	}
	in.sub.emit(e)
	e.WriteByte(')')
}

// ---------- BETWEEN ----------

type betweenP struct {
	left AnyExpr
	lo   any
	hi   any
}

func (betweenP) predicate() {}
func (b betweenP) Emit(e *Emitter) {
	b.left.Emit(e)
	e.WriteString(" BETWEEN ")
	e.Param(b.lo)
	e.WriteString(" AND ")
	e.Param(b.hi)
}

// ---------- IS NULL / IS NOT NULL ----------

type nullP struct {
	left AnyExpr
	not  bool
}

func (nullP) predicate() {}
func (n nullP) Emit(e *Emitter) {
	n.left.Emit(e)
	if n.not {
		e.WriteString(" IS NOT NULL")
	} else {
		e.WriteString(" IS NULL")
	}
}

// ---------- bare AnyExpr wrapped as predicate ----------

// AsPredicate wraps a boolean Expr[bool] as a Predicate. Useful for typed
// helpers returning Expr[bool] (e.g. `bitmapContains(b, uid)`) that need
// to slot into WHERE.
func AsPredicate(e Expr[bool]) Predicate { return exprPred{e: e} }

type exprPred struct{ e AnyExpr }

func (exprPred) predicate() {}
func (p exprPred) Emit(e *Emitter) {
	p.e.Emit(e)
}
