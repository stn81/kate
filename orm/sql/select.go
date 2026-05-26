package sql

// SelectBuilder is the immutable, flavor-agnostic SELECT builder. Each
// mutator returns a fresh value (slices are copy-on-write) so a base query
// can be safely shared across goroutines and forked into N variants.
//
// Dialect-specific clauses (CK FINAL / PREWHERE / SETTINGS / LIMIT BY) are
// not on this type; they live on *olap/ch.SelectBuilder, which embeds
// *sql.SelectBuilder. The CK variant injects its own clauses through
// EmitExtension hooks during Build.
type SelectBuilder struct {
	ctes     []*CTE
	distinct bool

	selectCols []AnyExpr
	selectAll  bool

	from TableRef
	hasFrom bool

	joins []joinTerm

	whereExprs   []Predicate
	groupByExprs []AnyExpr
	havingExprs  []Predicate
	orderBy      []OrderTerm

	limit     int64
	hasLimit  bool
	offset    int64
	hasOffset bool

	forUpdate bool

	// extension is invoked at emit time after the common clauses are
	// written, so dialect-specific builders (ch.SelectBuilder) can inject
	// FINAL / PREWHERE / SETTINGS / LIMIT BY without re-writing the whole
	// emit path. Implementations call the named methods on *Emitter and
	// reach into hook fields they own.
	extension SelectExtension
}

// SelectExtension is implemented by dialect-specific builders that wrap
// *SelectBuilder to inject extra clauses at well-defined emission stages.
type SelectExtension interface {
	// EmitAfterTable runs right after "FROM <table> [JOIN ...]" and before
	// WHERE. ClickHouse uses this for FINAL.
	EmitAfterTable(e *Emitter)
	// EmitBeforeWhere runs immediately before the WHERE keyword, after any
	// AfterTable emission. ClickHouse uses this for PREWHERE.
	EmitBeforeWhere(e *Emitter)
	// EmitAfterOrderBy runs after ORDER BY but before LIMIT. ClickHouse
	// uses this for LIMIT BY (which precedes a final LIMIT) — note that CK
	// LIMIT BY actually wraps differently; the ch builder is responsible
	// for handling order.
	EmitAfterOrderBy(e *Emitter)
	// EmitTrailing runs at the very end (after LIMIT/FOR UPDATE) for
	// SETTINGS-style suffixes.
	EmitTrailing(e *Emitter)
}

// From starts a SelectBuilder against a table source.
func From(t TableRef) *SelectBuilder {
	return &SelectBuilder{from: t, hasFrom: true}
}

// SelectNoTable is used when the SELECT has no FROM (e.g. CK
// `SELECT arrayJoin(bitmapToArray(bm))`).
func SelectNoTable() *SelectBuilder { return &SelectBuilder{} }

// WithExtension attaches a dialect-specific emit extension. Used internally
// by olap/ch.From; user code should not call this directly.
func (b *SelectBuilder) WithExtension(ext SelectExtension) *SelectBuilder {
	out := *b
	out.extension = ext
	return &out
}

// Clone makes a defensive copy. Mutating methods already clone slices
// internally so callers rarely need this; exposed for the ch upgrade path.
func (b *SelectBuilder) Clone() *SelectBuilder {
	out := *b
	out.ctes = append([]*CTE(nil), b.ctes...)
	out.selectCols = append([]AnyExpr(nil), b.selectCols...)
	out.joins = append([]joinTerm(nil), b.joins...)
	out.whereExprs = append([]Predicate(nil), b.whereExprs...)
	out.groupByExprs = append([]AnyExpr(nil), b.groupByExprs...)
	out.havingExprs = append([]Predicate(nil), b.havingExprs...)
	out.orderBy = append([]OrderTerm(nil), b.orderBy...)
	return &out
}

// ----- mutators (all immutable: return fresh *SelectBuilder) -----

// Select sets the projection list. Repeated calls replace; use SelectAdd
// to append.
func (b *SelectBuilder) Select(cols ...AnyExpr) *SelectBuilder {
	out := b.Clone()
	out.selectCols = append([]AnyExpr(nil), cols...)
	out.selectAll = false
	return out
}

// SelectAdd appends to the projection list.
func (b *SelectBuilder) SelectAdd(cols ...AnyExpr) *SelectBuilder {
	out := b.Clone()
	out.selectCols = append(out.selectCols, cols...)
	return out
}

// SelectAll marks the SELECT as `SELECT *`. Discouraged but available for
// ad-hoc exploration / RawExpr fallback.
func (b *SelectBuilder) SelectAll() *SelectBuilder {
	out := b.Clone()
	out.selectAll = true
	out.selectCols = nil
	return out
}

// Distinct marks the SELECT as DISTINCT.
func (b *SelectBuilder) Distinct() *SelectBuilder {
	out := b.Clone()
	out.distinct = true
	return out
}

// From replaces the source table.
func (b *SelectBuilder) From(t TableRef) *SelectBuilder {
	out := b.Clone()
	out.from = t
	out.hasFrom = true
	return out
}

// ---- joins ----

type joinKind int

const (
	innerJoin joinKind = iota
	leftJoin
	rightJoin
	fullJoin
	crossJoin
)

type joinTerm struct {
	kind joinKind
	tbl  TableRef
	on   Predicate
}

func (j joinKind) keyword() string {
	switch j {
	case leftJoin:
		return "LEFT JOIN"
	case rightJoin:
		return "RIGHT JOIN"
	case fullJoin:
		return "FULL JOIN"
	case crossJoin:
		return "CROSS JOIN"
	}
	return "JOIN"
}

// Join appends an INNER JOIN.
func (b *SelectBuilder) Join(t TableRef, on Predicate) *SelectBuilder {
	out := b.Clone()
	out.joins = append(out.joins, joinTerm{kind: innerJoin, tbl: t, on: on})
	return out
}

// LeftJoin appends a LEFT [OUTER] JOIN.
func (b *SelectBuilder) LeftJoin(t TableRef, on Predicate) *SelectBuilder {
	out := b.Clone()
	out.joins = append(out.joins, joinTerm{kind: leftJoin, tbl: t, on: on})
	return out
}

// RightJoin appends a RIGHT [OUTER] JOIN.
func (b *SelectBuilder) RightJoin(t TableRef, on Predicate) *SelectBuilder {
	out := b.Clone()
	out.joins = append(out.joins, joinTerm{kind: rightJoin, tbl: t, on: on})
	return out
}

// FullJoin appends a FULL [OUTER] JOIN.
func (b *SelectBuilder) FullJoin(t TableRef, on Predicate) *SelectBuilder {
	out := b.Clone()
	out.joins = append(out.joins, joinTerm{kind: fullJoin, tbl: t, on: on})
	return out
}

// CrossJoin appends a CROSS JOIN (no ON clause).
func (b *SelectBuilder) CrossJoin(t TableRef) *SelectBuilder {
	out := b.Clone()
	out.joins = append(out.joins, joinTerm{kind: crossJoin, tbl: t, on: nil})
	return out
}

// Where appends AND-joined predicates. Repeated calls append; never replace.
func (b *SelectBuilder) Where(ps ...Predicate) *SelectBuilder {
	out := b.Clone()
	out.whereExprs = append(out.whereExprs, ps...)
	return out
}

// GroupBy appends GROUP BY expressions.
func (b *SelectBuilder) GroupBy(es ...AnyExpr) *SelectBuilder {
	out := b.Clone()
	out.groupByExprs = append(out.groupByExprs, es...)
	return out
}

// Having appends AND-joined HAVING predicates.
func (b *SelectBuilder) Having(ps ...Predicate) *SelectBuilder {
	out := b.Clone()
	out.havingExprs = append(out.havingExprs, ps...)
	return out
}

// OrderBy appends ORDER BY terms (each carrying its own direction).
func (b *SelectBuilder) OrderBy(terms ...OrderTerm) *SelectBuilder {
	out := b.Clone()
	out.orderBy = append(out.orderBy, terms...)
	return out
}

// Limit sets the LIMIT (row cap).
func (b *SelectBuilder) Limit(n int64) *SelectBuilder {
	out := b.Clone()
	out.limit = n
	out.hasLimit = true
	return out
}

// Offset sets the OFFSET (skip rows).
func (b *SelectBuilder) Offset(n int64) *SelectBuilder {
	out := b.Clone()
	out.offset = n
	out.hasOffset = true
	return out
}

// With attaches a CTE. Multiple calls accumulate.
func (b *SelectBuilder) With(c *CTE) *SelectBuilder {
	out := b.Clone()
	out.ctes = append(out.ctes, c)
	return out
}

// WithMany attaches several CTEs at once.
func (b *SelectBuilder) WithMany(cs ...*CTE) *SelectBuilder {
	out := b.Clone()
	out.ctes = append(out.ctes, cs...)
	return out
}

// ForUpdate appends `FOR UPDATE`. ClickHouse silently drops + warns
// (see flavor capabilities); MySQL/PG emit verbatim.
func (b *SelectBuilder) ForUpdate() *SelectBuilder {
	out := b.Clone()
	out.forUpdate = true
	return out
}

// ----- terminal -----

// Build compiles the SELECT against the given flavor.
func (b *SelectBuilder) Build(f Flavor) (string, []any, error) {
	e := NewEmitter(f)
	b.emit(e)
	return e.Result()
}

// emit writes the SELECT into the given emitter (sharing args). This is the
// internal entry point used by CTE inlining and by *ch.SelectBuilder's
// Build, which calls back into here after attaching its extension.
func (b *SelectBuilder) emit(e *Emitter) {
	emitCTEs(e, b.ctes)

	e.WriteString("SELECT ")
	if b.distinct {
		e.WriteString("DISTINCT ")
	}
	switch {
	case b.selectAll:
		e.WriteByte('*')
	case len(b.selectCols) == 0:
		// no projection — emit `1` to keep the SQL syntactically valid
		// (used by EXISTS subqueries built ad-hoc)
		e.WriteByte('1')
	default:
		for i, c := range b.selectCols {
			if i > 0 {
				e.WriteString(", ")
			}
			c.Emit(e)
		}
	}

	if b.hasFrom && !(b.from.virtual && b.from.alias == "") {
		e.WriteString(" FROM ")
		b.from.emitSource(e)
	}

	for _, j := range b.joins {
		e.WriteByte(' ')
		e.WriteString(j.kind.keyword())
		e.WriteByte(' ')
		j.tbl.emitSource(e)
		if j.on != nil {
			e.WriteString(" ON ")
			j.on.Emit(e)
		}
	}

	if b.extension != nil {
		b.extension.EmitAfterTable(e)
		b.extension.EmitBeforeWhere(e)
	}

	if len(b.whereExprs) > 0 {
		e.WriteString(" WHERE ")
		andP(b.whereExprs).Emit(e)
	}

	if len(b.groupByExprs) > 0 {
		e.WriteString(" GROUP BY ")
		for i, g := range b.groupByExprs {
			if i > 0 {
				e.WriteString(", ")
			}
			g.Emit(e)
		}
	}

	if len(b.havingExprs) > 0 {
		e.WriteString(" HAVING ")
		andP(b.havingExprs).Emit(e)
	}

	if len(b.orderBy) > 0 {
		e.WriteString(" ORDER BY ")
		for i, o := range b.orderBy {
			if i > 0 {
				e.WriteString(", ")
			}
			o.emit(e)
		}
	}

	if b.extension != nil {
		b.extension.EmitAfterOrderBy(e)
	}

	if b.hasLimit {
		e.WriteString(" LIMIT ")
		writeInt(e, b.limit)
	}
	if b.hasOffset {
		e.WriteString(" OFFSET ")
		writeInt(e, b.offset)
	}

	if b.forUpdate {
		// PG / MySQL: emit verbatim. CK: drop silently (no row-level locks).
		if e.flavor.Name() != "ClickHouse" {
			e.WriteString(" FOR UPDATE")
		}
	}

	if b.extension != nil {
		b.extension.EmitTrailing(e)
	}
}

func writeInt(e *Emitter, n int64) {
	// avoid pulling in strconv to keep the emit hot path tight
	if n == 0 {
		e.WriteByte('0')
		return
	}
	if n < 0 {
		e.WriteByte('-')
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	e.WriteString(string(buf[i:]))
}

// Inner returns the SelectBuilder itself — used as a clarity marker when
// passing a sub-SelectBuilder into InSubquery / Subquery to make the
// "this is a nested SELECT" intent explicit at the call site.
func (b *SelectBuilder) Inner() *SelectBuilder { return b }

// SelectColCount returns the number of projected columns (for the subquery
// arity check). Returns -1 for SELECT *.
func (b *SelectBuilder) SelectColCount() int {
	if b.selectAll {
		return -1
	}
	return len(b.selectCols)
}
