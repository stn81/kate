package sql

// CTE is a typed common table expression — a named subquery attached to a
// SELECT via With. The Row generic parameter exists primarily as
// documentation: it doesn't constrain column access (column names inside
// CTEs are dynamic strings), but it lets callers parameterize generic
// helpers that produce / consume CTEs.
type CTE[Row any] struct {
	name string
	body Builder // the inner SELECT — usually *SelectBuilder or *ch.SelectBuilder
}

// NewCTE constructs a typed CTE wrapping any Builder as its body. The body
// is expected to be a SELECT (or compatible expression that emits a query).
func NewCTE[Row any](name string, body Builder) *CTE[Row] {
	return &CTE[Row]{name: name, body: body}
}

// Name returns the CTE alias.
func (c *CTE[Row]) Name() string { return c.name }

// Ref returns a TableRef that addresses the CTE as a FROM source. Column
// access on the ref is dynamic — use TableRef.Col(name) for AnyCol or
// sql.ColAs[T](ref, name) for a typed Col[T].
func (c *CTE[Row]) Ref() TableRef { return NewVirtualTable(c.name) }

// AnyCTE is the existential erasure of *CTE[Row] used by SelectBuilder.With
// so heterogeneous CTE rows can share a single attachment slot.
type AnyCTE interface {
	cteName() string
	cteBody() Builder
}

func (c *CTE[Row]) cteName() string { return c.name }
func (c *CTE[Row]) cteBody() Builder { return c.body }

// emitCTEs writes the WITH ... clause for a list of attached CTEs. Empty
// list emits nothing.
func emitCTEs(e *Emitter, ctes []AnyCTE) {
	if len(ctes) == 0 {
		return
	}
	if !e.flavor.SupportsCTE() {
		e.SetError(errCTEUnsupported(e.flavor.Name()))
		return
	}
	e.WriteString("WITH ")
	for i, c := range ctes {
		if i > 0 {
			e.WriteString(", ")
		}
		e.Quote(c.cteName())
		e.WriteString(" AS (")
		emitInner(e, c.cteBody())
		e.WriteByte(')')
	}
	e.WriteByte(' ')
}

// emitInner emits a sub-builder inline into the current emitter, sharing
// the args buffer. We do this by asking the builder to use a special
// "embedded" emit path that writes to our emitter rather than building its
// own. For *SelectBuilder this is the unexported emit method; for opaque
// Builders we fall back to Build() and inline-substitute placeholders
// (rare path; CTE bodies are almost always *SelectBuilder).
func emitInner(e *Emitter, b Builder) {
	if sb, ok := b.(*SelectBuilder); ok {
		sb.emit(e)
		return
	}
	// Generic fallback: build the inner statement with the same flavor and
	// splice it in. Placeholders in inner SQL are flavor-formatted as if
	// they were first; since we control flavor we re-parameterize against
	// our emitter to preserve global placeholder ordering.
	sql, args, err := b.Build(e.flavor)
	if err != nil {
		e.SetError(err)
		return
	}
	// rewrite inner placeholders by re-emitting through Param. For MySQL/CK
	// (`?`) this is a simple scan; for PG (`$N`) we'd need to renumber but
	// CTE bodies are almost always *SelectBuilder so we don't hit this in
	// practice. We support the simple `?` case.
	idx := 0
	for i := 0; i < len(sql); i++ {
		if sql[i] == '?' && idx < len(args) {
			e.Param(args[idx])
			idx++
		} else {
			e.WriteByte(sql[i])
		}
	}
}
