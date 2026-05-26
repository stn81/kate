// Package oltp provides MySQL/PostgreSQL row-level write builders. Builders
// in this package construct statements that assume row-level INSERT /
// UPDATE / DELETE semantics; they explicitly reject ClickHouse flavors at
// Exec time (CK does not support row-scope UPDATE / DELETE — use the olap
// package's MutateBuilder for that).
package oltp

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/stn81/kate/orm/db"
	ksql "github.com/stn81/kate/orm/sql"
)

// InsertBuilder[T] builds INSERT statements that consume T-typed row values.
// Construct via Into[T](table); add rows via Values; commit with Exec.
type InsertBuilder[T any] struct {
	table     ksql.TableRef
	rows      []T
	columns   []string // resolved at first Values call from the row struct tags
	returning []ksql.AnyExpr
	conflict  *conflictClause
}

// Into starts an InsertBuilder against the given table.
func Into[T any](t ksql.TableRef) *InsertBuilder[T] {
	return &InsertBuilder[T]{table: t}
}

// Values appends row values to be inserted. The struct field → column
// mapping is resolved on the first call via reflection (`db:"col"` /
// `orm:"column(col)"` tags), then cached.
func (b *InsertBuilder[T]) Values(rows ...T) *InsertBuilder[T] {
	out := *b
	out.rows = append(append([]T(nil), b.rows...), rows...)
	return &out
}

// Returning sets a RETURNING clause (PostgreSQL only). Build emits an
// error against non-PG flavors.
func (b *InsertBuilder[T]) Returning(cols ...ksql.AnyExpr) *InsertBuilder[T] {
	out := *b
	out.returning = append([]ksql.AnyExpr(nil), cols...)
	return &out
}

// conflictClause is the parsed ON CONFLICT / ON DUPLICATE KEY config.
type conflictClause struct {
	target ksql.AnyCol         // PG: ON CONFLICT (target)
	update map[string]ksql.AnyExpr // col → new value (excluded.col for PG, VALUES(col) for MySQL)
	ignore bool                // true = DO NOTHING / IGNORE
}

// ConflictClause is the partial-builder returned by OnConflict, used to
// further configure the conflict resolution.
type ConflictClause[T any] struct {
	parent *InsertBuilder[T]
	cl     conflictClause
}

// OnConflict starts an ON CONFLICT (target) clause. PG and MySQL emit
// different SQL but the semantics are equivalent for the common
// "upsert by primary key" case.
func (b *InsertBuilder[T]) OnConflict(target ksql.AnyCol) *ConflictClause[T] {
	return &ConflictClause[T]{parent: b, cl: conflictClause{target: target, update: map[string]ksql.AnyExpr{}}}
}

// DoNothing finishes a conflict clause with no-op resolution.
func (c *ConflictClause[T]) DoNothing() *InsertBuilder[T] {
	c.cl.ignore = true
	out := *c.parent
	out.conflict = &c.cl
	return &out
}

// DoUpdate sets a single col → new-value pair on conflict.
func (c *ConflictClause[T]) DoUpdate(col ksql.AnyCol, val ksql.AnyExpr) *ConflictClause[T] {
	if c.cl.update == nil {
		c.cl.update = map[string]ksql.AnyExpr{}
	}
	c.cl.update[columnNameOf(col)] = val
	return c
}

// Finish closes the conflict clause and returns the parent builder.
func (c *ConflictClause[T]) Finish() *InsertBuilder[T] {
	out := *c.parent
	out.conflict = &c.cl
	return &out
}

// Build compiles the INSERT statement.
func (b *InsertBuilder[T]) Build(f ksql.Flavor) (string, []any, error) {
	if len(b.rows) == 0 {
		return "", nil, fmt.Errorf("kate/oltp: InsertBuilder has no Values")
	}
	cols, err := columnsForType[T]()
	if err != nil {
		return "", nil, err
	}
	em := ksql.NewEmitter(f)
	em.WriteString("INSERT INTO ")
	emitTableNoAlias(em, b.table, f)
	_ = em.WriteByte(' ')
	_ = em.WriteByte('(')
	for i, c := range cols.names {
		if i > 0 {
			em.WriteString(", ")
		}
		em.WriteString(f.Quote(c))
	}
	em.WriteString(") VALUES ")
	for ri, row := range b.rows {
		if ri > 0 {
			em.WriteString(", ")
		}
		_ = em.WriteByte('(')
		rv := reflect.ValueOf(row)
		for ci, path := range cols.fieldPaths {
			if ci > 0 {
				em.WriteString(", ")
			}
			fv := rv
			for _, idx := range path {
				if fv.Kind() == reflect.Pointer {
					if fv.IsNil() {
						em.Param(nil)
						goto next
					}
					fv = fv.Elem()
				}
				fv = fv.Field(idx)
			}
			em.Param(fv.Interface())
		next:
		}
		_ = em.WriteByte(')')
	}
	if b.conflict != nil {
		emitConflict(em, f, cols, b.conflict)
	}
	if len(b.returning) > 0 {
		if !f.SupportsReturning() {
			return "", nil, fmt.Errorf("kate/oltp: flavor %q does not support RETURNING", f.Name())
		}
		em.WriteString(" RETURNING ")
		for i, e := range b.returning {
			if i > 0 {
				em.WriteString(", ")
			}
			e.Emit(em)
		}
	}
	return em.Result()
}

// Exec compiles and runs the INSERT against the executor. Returns the
// db.Result (LastInsertId / RowsAffected). On a ClickHouse flavor returns
// ErrFlavorMismatch — use olap.BatchInserter for CK writes.
func (b *InsertBuilder[T]) Exec(ctx context.Context, exe db.Executor) (db.Result, error) {
	if db.FlavorOf(exe).Name() == "ClickHouse" {
		return db.Result{}, db.ErrFlavorMismatch
	}
	return db.ExecBuilder(ctx, exe, b)
}

// emitConflict writes the flavor-specific conflict resolution clause.
func emitConflict(em *ksql.Emitter, f ksql.Flavor, cols *columnSet, c *conflictClause) {
	switch f.Name() {
	case "PostgreSQL":
		em.WriteString(" ON CONFLICT (")
		c.target.Emit(em)
		em.WriteString(")")
		if c.ignore {
			em.WriteString(" DO NOTHING")
			return
		}
		em.WriteString(" DO UPDATE SET ")
		keys := sortedKeys(c.update)
		for i, k := range keys {
			if i > 0 {
				em.WriteString(", ")
			}
			em.WriteString(f.Quote(k))
			em.WriteString(" = ")
			c.update[k].Emit(em)
		}
	case "MySQL":
		if c.ignore {
			// MySQL flavor uses INSERT IGNORE, which would need an
			// earlier-stage hook; for now we emit ON DUPLICATE KEY UPDATE
			// no-op (id=id) using the first column.
			if len(cols.names) > 0 {
				em.WriteString(" ON DUPLICATE KEY UPDATE ")
				em.WriteString(f.Quote(cols.names[0]))
				em.WriteString(" = ")
				em.WriteString(f.Quote(cols.names[0]))
			}
			return
		}
		em.WriteString(" ON DUPLICATE KEY UPDATE ")
		keys := sortedKeys(c.update)
		for i, k := range keys {
			if i > 0 {
				em.WriteString(", ")
			}
			em.WriteString(f.Quote(k))
			em.WriteString(" = ")
			c.update[k].Emit(em)
		}
	}
}

func sortedKeys(m map[string]ksql.AnyExpr) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// columnNameOf extracts the bare column name from an AnyCol. We dont have
// a typed accessor; emit it into a throwaway emitter using the MySQL
// flavor and strip the backticks.
func columnNameOf(c ksql.AnyCol) string {
	em := ksql.NewEmitter(stripFlavor{})
	c.Emit(em)
	s, _, _ := em.Result()
	// strip any "table.col" prefix and trailing alias
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, " AS "); i >= 0 {
		s = s[:i]
	}
	return s
}

// stripFlavor is a flavor that emits identifiers with no quotes (used by
// columnNameOf to recover a bare column name from an AnyCol's emit).
type stripFlavor struct{}

func (stripFlavor) Name() string                  { return "strip" }
func (stripFlavor) Quote(s string) string         { return s }
func (stripFlavor) Placeholder(i int) string      { return "?" }
func (stripFlavor) SupportsCTE() bool             { return true }
func (stripFlavor) SupportsReturning() bool       { return true }

func emitTableNoAlias(em *ksql.Emitter, t ksql.TableRef, f ksql.Flavor) {
	if t.Schema() != "" {
		em.WriteString(f.Quote(t.Schema()))
		_ = em.WriteByte('.')
	}
	em.WriteString(f.Quote(t.Name()))
}
