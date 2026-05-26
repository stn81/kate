package sql

// CTE is a named subquery attached to a SELECT via With. Column access on
// the CTE's TableRef is dynamic — use ref.Col(name) (untyped) or
// sql.ColAs[T](ref, name) (typed) to read columns.
type CTE struct {
	name string
	body Builder
}

// NewCTE constructs a CTE wrapping any Builder as its body. Body is usually
// a *SelectBuilder or *ch.SelectBuilder.
func NewCTE(name string, body Builder) *CTE { return &CTE{name: name, body: body} }

// Name returns the CTE alias.
func (c *CTE) Name() string { return c.name }

// Ref returns a TableRef that addresses the CTE as a FROM source.
func (c *CTE) Ref() TableRef { return NewVirtualTable(c.name) }

// emitCTEs writes WITH name AS (...), name AS (...) for attached CTEs.
func emitCTEs(e *Emitter, ctes []*CTE) {
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
		e.Quote(c.name)
		e.WriteString(" AS (")
		emitInner(e, c.body)
		_ = e.WriteByte(')')
	}
	_ = e.WriteByte(' ')
}

// emitInner inlines a sub-builder into the outer emitter, sharing args.
// Fast path: *SelectBuilder.emit reuses the same buffer. Other Builder
// implementations fall back to Build() + placeholder rewrite.
func emitInner(e *Emitter, b Builder) {
	if sb, ok := b.(*SelectBuilder); ok {
		sb.emit(e)
		return
	}
	s, args, err := b.Build(e.flavor)
	if err != nil {
		e.SetError(err)
		return
	}
	idx := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '?' && idx < len(args) {
			e.Param(args[idx])
			idx++
		} else {
			_ = e.WriteByte(s[i])
		}
	}
}
