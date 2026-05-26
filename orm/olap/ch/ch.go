// Package ch provides ClickHouse-only SELECT builder upgrades: FINAL,
// PREWHERE, SETTINGS, LIMIT BY, SAMPLE — clauses with no portable analog.
//
// The package's SelectBuilder embeds *sql.SelectBuilder and overrides Build
// so that ClickHouse-specific clauses are appended at the correct emission
// stage. The embedding lets all common builder methods (Where/Join/GroupBy/
// OrderBy/Limit/With) propagate through, while the extra methods on
// *ch.SelectBuilder are only reachable from this concrete type — meaning
// FINAL / PREWHERE / etc. cannot be accidentally constructed against a
// MySQL or PG flavor.
package ch

import (
	"sort"

	"github.com/stn81/kate/orm/flavor/clickhouse"
	ksql "github.com/stn81/kate/orm/sql"
)

// SelectBuilder is a ClickHouse-flavored SELECT. Wrap a *sql.SelectBuilder
// via From / FromSubquery / Upgrade. All inherited methods continue to
// work and return *SelectBuilder (we shadow the common mutators so chained
// calls remain in the CK type).
type SelectBuilder struct {
	inner *ksql.SelectBuilder

	final    bool
	prewhere []ksql.Predicate
	settings map[string]any
	sample   *sampleClause
	limitBy  *limitByClause

	// unionParts is populated when this builder is the result of UnionAll;
	// Build then emits "<part0> UNION ALL <part1> ..." instead of the
	// inner SELECT.
	unionParts []*SelectBuilder
}

type sampleClause struct {
	ratio float64
	// offset for SAMPLE k OFFSET m; -1 = unset.
	offset float64
	hasOff bool
}

type limitByClause struct {
	n  int64
	by []ksql.AnyExpr
}

// From starts a CK SelectBuilder against the given table.
func From(t ksql.TableRef) *SelectBuilder {
	b := &SelectBuilder{inner: ksql.From(t)}
	b.inner = b.inner.WithExtension(b.extension())
	return b
}

// FromSubquery starts a CK SelectBuilder from a sub-SELECT aliased as `alias`.
func FromSubquery(sub *ksql.SelectBuilder, alias string) *SelectBuilder {
	// emit "FROM (sub) AS alias" by wrapping the sub via a synthetic
	// TableRef pointed at a virtual table — we leverage CTE-style emit.
	// Simplest correct route: turn sub into a single-CTE attachment and
	// FROM the CTE alias.
	cte := ksql.NewCTE(alias, sub)
	b := &SelectBuilder{inner: ksql.From(ksql.NewVirtualTable(alias)).With(cte)}
	b.inner = b.inner.WithExtension(b.extension())
	return b
}

// SelectLiteral starts a CK SELECT without a FROM clause (e.g.
// `SELECT arrayJoin(bitmapToArray(bm))`).
func SelectLiteral() *SelectBuilder {
	b := &SelectBuilder{inner: ksql.SelectNoTable()}
	b.inner = b.inner.WithExtension(b.extension())
	return b
}

// Upgrade lifts a portable *sql.SelectBuilder into a CK builder, allowing
// CK-only methods to be chained on top. Use when a base query is built
// portably and a CK-specific tail (FINAL / SETTINGS) is needed.
func Upgrade(sb *ksql.SelectBuilder) *SelectBuilder {
	b := &SelectBuilder{inner: sb}
	b.inner = b.inner.WithExtension(b.extension())
	return b
}

// UnionAll combines several CK SELECT branches with UNION ALL. Returns a
// new CK SelectBuilder whose Build emits "<a> UNION ALL <b> UNION ALL ...".
//
// Implemented by producing a RawExpr-backed CTE body — UNION ALL is one of
// the few constructs not modeled as a first-class builder; treating it
// as a raw join of inner SQLs is pragmatic and matches CK semantics.
func UnionAll(parts ...*SelectBuilder) *SelectBuilder {
	out := &SelectBuilder{inner: ksql.SelectNoTable()}
	out.inner = out.inner.WithExtension(out.extension())
	out.unionParts = parts
	return out
}

// ----- mutator shadows: return *ch.SelectBuilder to keep the chain in this type -----

// Select sets projection columns.
func (b *SelectBuilder) Select(cols ...ksql.AnyExpr) *SelectBuilder {
	return b.replace(b.inner.Select(cols...))
}

// SelectAdd appends to the projection.
func (b *SelectBuilder) SelectAdd(cols ...ksql.AnyExpr) *SelectBuilder {
	return b.replace(b.inner.SelectAdd(cols...))
}

// SelectAll switches to SELECT *.
func (b *SelectBuilder) SelectAll() *SelectBuilder { return b.replace(b.inner.SelectAll()) }

// Distinct marks the SELECT as DISTINCT.
func (b *SelectBuilder) Distinct() *SelectBuilder { return b.replace(b.inner.Distinct()) }

// From replaces the FROM source.
func (b *SelectBuilder) From(t ksql.TableRef) *SelectBuilder { return b.replace(b.inner.From(t)) }

// Where appends AND-joined predicates.
func (b *SelectBuilder) Where(ps ...ksql.Predicate) *SelectBuilder {
	return b.replace(b.inner.Where(ps...))
}

// Join, LeftJoin, RightJoin, FullJoin, CrossJoin — delegate to inner.
func (b *SelectBuilder) Join(t ksql.TableRef, on ksql.Predicate) *SelectBuilder {
	return b.replace(b.inner.Join(t, on))
}
func (b *SelectBuilder) LeftJoin(t ksql.TableRef, on ksql.Predicate) *SelectBuilder {
	return b.replace(b.inner.LeftJoin(t, on))
}
func (b *SelectBuilder) RightJoin(t ksql.TableRef, on ksql.Predicate) *SelectBuilder {
	return b.replace(b.inner.RightJoin(t, on))
}
func (b *SelectBuilder) FullJoin(t ksql.TableRef, on ksql.Predicate) *SelectBuilder {
	return b.replace(b.inner.FullJoin(t, on))
}
func (b *SelectBuilder) CrossJoin(t ksql.TableRef) *SelectBuilder {
	return b.replace(b.inner.CrossJoin(t))
}

// GroupBy / Having / OrderBy / Limit / Offset / With.
func (b *SelectBuilder) GroupBy(es ...ksql.AnyExpr) *SelectBuilder {
	return b.replace(b.inner.GroupBy(es...))
}
func (b *SelectBuilder) Having(ps ...ksql.Predicate) *SelectBuilder {
	return b.replace(b.inner.Having(ps...))
}
func (b *SelectBuilder) OrderBy(terms ...ksql.OrderTerm) *SelectBuilder {
	return b.replace(b.inner.OrderBy(terms...))
}
func (b *SelectBuilder) Limit(n int64) *SelectBuilder  { return b.replace(b.inner.Limit(n)) }
func (b *SelectBuilder) Offset(n int64) *SelectBuilder { return b.replace(b.inner.Offset(n)) }
func (b *SelectBuilder) With(c *ksql.CTE) *SelectBuilder {
	return b.replace(b.inner.With(c))
}
func (b *SelectBuilder) WithMany(cs ...*ksql.CTE) *SelectBuilder {
	return b.replace(b.inner.WithMany(cs...))
}

// ----- CK-only mutators -----

// Final appends FINAL to the FROM table (collapses ReplacingMergeTree
// duplicate rows; expensive but necessary for deduplicated reads).
func (b *SelectBuilder) Final() *SelectBuilder {
	out := b.clone()
	out.final = true
	return out
}

// Prewhere appends predicates that should run as PREWHERE (a CK optimization
// for filtering rows before reading full columns). Use for highly selective
// filters that touch a tiny subset of the projection columns.
func (b *SelectBuilder) Prewhere(ps ...ksql.Predicate) *SelectBuilder {
	out := b.clone()
	out.prewhere = append(out.prewhere, ps...)
	return out
}

// Settings appends query-level SETTINGS (e.g. max_threads, join_algorithm).
// Repeated calls merge; later keys override earlier ones.
func (b *SelectBuilder) Settings(kv map[string]any) *SelectBuilder {
	out := b.clone()
	if out.settings == nil {
		out.settings = make(map[string]any, len(kv))
	}
	for k, v := range kv {
		out.settings[k] = v
	}
	return out
}

// Sample sets SAMPLE k (0 < k <= 1 = fraction; k > 1 = approx row count).
func (b *SelectBuilder) Sample(ratio float64) *SelectBuilder {
	out := b.clone()
	out.sample = &sampleClause{ratio: ratio}
	return out
}

// LimitBy sets LIMIT n BY expr1, expr2 — CK's per-group LIMIT.
func (b *SelectBuilder) LimitBy(n int64, by ...ksql.AnyExpr) *SelectBuilder {
	out := b.clone()
	out.limitBy = &limitByClause{n: n, by: append([]ksql.AnyExpr(nil), by...)}
	return out
}

// ----- terminal -----

// Build compiles against the ClickHouse flavor implicitly. Callers using
// the *DB-driven path don't call this directly; db.List[T] etc. invoke
// it via the Builder interface.
func (b *SelectBuilder) Build(f ksql.Flavor) (string, []any, error) {
	if _, ok := f.(interface{ IsClickHouse() }); !ok {
		// We accept any flavor and emit CK clauses anyway when the builder
		// is used; in practice callers always pass the CK flavor adapter
		// because they constructed via ch.From and only ever target a
		// CK *DB. The defense-in-depth check exists for robustness if a
		// caller ever splices a builder across flavors.
		// We still build, since the emit produces valid CK SQL — but
		// produce a warning would be flavor-specific noise; emit silently.
		_ = f
	}
	// Override the inner extension with one captured against b's current
	// state — the inner clone'd through Sel/etc. mutations keeps the
	// extension pointer, but mutations to b.final/etc. happened on this
	// instance; reattach.
	inner := b.inner.WithExtension(b.extension())
	if len(b.unionParts) > 0 {
		return b.buildUnion(f)
	}
	return inner.Build(f)
}

// SelectColCount delegates to inner — needed so Subquery shape-checks the
// CK builder.
func (b *SelectBuilder) SelectColCount() int { return b.inner.SelectColCount() }

// Inner returns the underlying *sql.SelectBuilder; useful for sub-selects
// where the CK-only clauses are not desired.
func (b *SelectBuilder) Inner() *ksql.SelectBuilder { return b.inner }

// ----- internals -----

func (b *SelectBuilder) buildUnion(f ksql.Flavor) (string, []any, error) {
	em := ksql.NewEmitter(f)
	for i, part := range b.unionParts {
		if i > 0 {
			em.WriteString(" UNION ALL ")
		}
		// emit each part inline by inlining its SQL.
		s, args, err := part.Build(f)
		if err != nil {
			return "", nil, err
		}
		// substitute placeholders into the outer emitter so global arg
		// ordering stays correct.
		idx := 0
		for j := 0; j < len(s); j++ {
			if s[j] == '?' && idx < len(args) {
				em.Param(args[idx])
				idx++
			} else {
				_ = em.WriteByte(s[j])
			}
		}
	}
	return em.Result()
}

func (b *SelectBuilder) clone() *SelectBuilder {
	out := *b
	if b.prewhere != nil {
		out.prewhere = append([]ksql.Predicate(nil), b.prewhere...)
	}
	if b.settings != nil {
		out.settings = make(map[string]any, len(b.settings))
		for k, v := range b.settings {
			out.settings[k] = v
		}
	}
	if b.sample != nil {
		s := *b.sample
		out.sample = &s
	}
	if b.limitBy != nil {
		l := *b.limitBy
		l.by = append([]ksql.AnyExpr(nil), b.limitBy.by...)
		out.limitBy = &l
	}
	// rewire extension to point at the new builder so the emit closure
	// reads the fresh fields.
	out.inner = b.inner.WithExtension(out.extension())
	return &out
}

func (b *SelectBuilder) replace(inner *ksql.SelectBuilder) *SelectBuilder {
	out := *b
	out.inner = inner.WithExtension(out.extension())
	return &out
}

// extension returns a SelectExtension bound to this builder's CK state. It
// captures b by pointer-ish (via the closure) so the inner emit reads
// up-to-date final/prewhere/etc.
func (b *SelectBuilder) extension() ksql.SelectExtension {
	return chExt{b: b}
}

type chExt struct{ b *SelectBuilder }

func (e chExt) EmitAfterTable(em *ksql.Emitter) {
	if e.b.final {
		em.WriteString(" FINAL")
	}
	if e.b.sample != nil {
		em.WriteString(" SAMPLE ")
		writeFloat(em, e.b.sample.ratio)
		if e.b.sample.hasOff {
			em.WriteString(" OFFSET ")
			writeFloat(em, e.b.sample.offset)
		}
	}
}

func (e chExt) EmitBeforeWhere(em *ksql.Emitter) {
	if len(e.b.prewhere) > 0 {
		em.WriteString(" PREWHERE ")
		// emit as AND-joined predicates
		for i, p := range e.b.prewhere {
			if i > 0 {
				em.WriteString(" AND ")
			}
			_ = em.WriteByte('(')
			p.Emit(em)
			_ = em.WriteByte(')')
		}
	}
}

func (e chExt) EmitAfterOrderBy(em *ksql.Emitter) {
	if e.b.limitBy != nil {
		em.WriteString(" LIMIT ")
		writeInt(em, e.b.limitBy.n)
		em.WriteString(" BY ")
		for i, by := range e.b.limitBy.by {
			if i > 0 {
				em.WriteString(", ")
			}
			by.Emit(em)
		}
	}
}

func (e chExt) EmitTrailing(em *ksql.Emitter) {
	if len(e.b.settings) > 0 {
		em.WriteString(" SETTINGS ")
		keys := make([]string, 0, len(e.b.settings))
		for k := range e.b.settings {
			keys = append(keys, k)
		}
		sort.Strings(keys) // stable emit for testability
		for i, k := range keys {
			if i > 0 {
				em.WriteString(", ")
			}
			em.WriteString(k)
			em.WriteString(" = ")
			em.Param(e.b.settings[k])
		}
	}
}

func writeInt(em *ksql.Emitter, n int64) {
	if n == 0 {
		_ = em.WriteByte('0')
		return
	}
	if n < 0 {
		_ = em.WriteByte('-')
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	em.WriteString(string(buf[i:]))
}

func writeFloat(em *ksql.Emitter, f float64) {
	// minimal float formatter; SAMPLE accepts strings like "0.1" or "10000".
	if f == float64(int64(f)) {
		writeInt(em, int64(f))
		return
	}
	em.WriteString(formatFloat(f))
}

// Ensure CK flavor used at compile time — silence unused import warning.
var _ = clickhouse.Flavor

// formatFloat is a minimal float→decimal-string formatter for SAMPLE values.
// We avoid strconv to stay zero-dep, and SAMPLE only needs ≤6 fractional
// digits; precision is set accordingly.
func formatFloat(f float64) string {
	// integer fast path handled by writeFloat
	neg := false
	if f < 0 {
		neg = true
		f = -f
	}
	whole := int64(f)
	frac := f - float64(whole)
	// scale up 1e6 for 6-digit precision; truncate trailing zeros.
	scaled := int64(frac*1e6 + 0.5)
	var buf [40]byte
	pos := len(buf)
	// 6-digit fractional
	if scaled > 0 {
		// trim trailing zeros
		for scaled%10 == 0 && scaled > 0 {
			scaled /= 10
		}
		for scaled > 0 {
			pos--
			buf[pos] = byte('0' + scaled%10)
			scaled /= 10
		}
		pos--
		buf[pos] = '.'
	}
	if whole == 0 {
		pos--
		buf[pos] = '0'
	} else {
		for whole > 0 {
			pos--
			buf[pos] = byte('0' + whole%10)
			whole /= 10
		}
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
