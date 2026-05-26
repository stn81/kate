package oltp

import (
	"context"
	"fmt"
	"sort"

	"github.com/stn81/kate/orm/db"
	ksql "github.com/stn81/kate/orm/sql"
)

// UpdateBuilder builds UPDATE statements for OLTP flavors. Construct via
// Update(table); add assignments with Set or SetTyped; constrain with
// Where; commit with Exec.
//
// Assignments are stored in insertion order so emit is deterministic.
type UpdateBuilder struct {
	table    ksql.TableRef
	setCols  []string         // column names in insertion order
	setExprs map[string]ksql.AnyExpr
	where    []ksql.Predicate
}

// Update starts an UpdateBuilder.
func Update(t ksql.TableRef) *UpdateBuilder {
	return &UpdateBuilder{table: t, setExprs: map[string]ksql.AnyExpr{}}
}

// Set sets col = expr. The typed alternative SetTyped[T] should be
// preferred when col and expr's type are statically known.
func (b *UpdateBuilder) Set(col ksql.AnyCol, expr ksql.AnyExpr) *UpdateBuilder {
	out := b.clone()
	name := columnNameOf(col)
	if _, exists := out.setExprs[name]; !exists {
		out.setCols = append(out.setCols, name)
	}
	out.setExprs[name] = expr
	return out
}

// Where appends AND-joined predicates.
func (b *UpdateBuilder) Where(ps ...ksql.Predicate) *UpdateBuilder {
	out := b.clone()
	out.where = append(out.where, ps...)
	return out
}

// Build compiles the UPDATE.
func (b *UpdateBuilder) Build(f ksql.Flavor) (string, []any, error) {
	if len(b.setCols) == 0 {
		return "", nil, fmt.Errorf("kate/oltp.Update: no Set / SetTyped calls")
	}
	em := ksql.NewEmitter(f)
	em.WriteString("UPDATE ")
	emitTableNoAlias(em, b.table, f)
	em.WriteString(" SET ")
	for i, col := range b.setCols {
		if i > 0 {
			em.WriteString(", ")
		}
		em.WriteString(f.Quote(col))
		em.WriteString(" = ")
		b.setExprs[col].Emit(em)
	}
	if len(b.where) > 0 {
		em.WriteString(" WHERE ")
		emitAndPreds(em, b.where)
	}
	return em.Result()
}

// Exec compiles and runs. CK flavor → ErrFlavorMismatch.
func (b *UpdateBuilder) Exec(ctx context.Context, exe db.Executor) (db.Result, error) {
	if db.FlavorOf(exe).Name() == "ClickHouse" {
		return db.Result{}, db.ErrFlavorMismatch
	}
	return db.ExecBuilder(ctx, exe, b)
}

func (b *UpdateBuilder) clone() *UpdateBuilder {
	out := *b
	out.setCols = append([]string(nil), b.setCols...)
	out.setExprs = make(map[string]ksql.AnyExpr, len(b.setExprs))
	for k, v := range b.setExprs {
		out.setExprs[k] = v
	}
	out.where = append([]ksql.Predicate(nil), b.where...)
	return &out
}

func emitAndPreds(em *ksql.Emitter, ps []ksql.Predicate) {
	for i, p := range ps {
		if i > 0 {
			em.WriteString(" AND ")
		}
		_ = em.WriteByte('(')
		p.Emit(em)
		_ = em.WriteByte(')')
	}
}

// SetTyped is the typed entry point. col is Col[T], value is T;
// compile-error if value doesn't match T. Use this for everyday updates.
func SetTyped[T any](b *UpdateBuilder, col ksql.Col[T], v T) *UpdateBuilder {
	return b.Set(col, ksql.Lit(v))
}

// SetExprTyped is the expr-on-RHS typed variant: col = expr where both
// sides are Expr[T]. Useful for "col = col + 1" style updates.
func SetExprTyped[T any](b *UpdateBuilder, col ksql.Col[T], e ksql.Expr[T]) *UpdateBuilder {
	return b.Set(col, e)
}

// Force determinism on map iteration in tests by sorting set columns;
// available as a build option, not exported.
//
//nolint:unused // utility for future deterministic-emit option
func sortedSetCols(m map[string]ksql.AnyExpr) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
